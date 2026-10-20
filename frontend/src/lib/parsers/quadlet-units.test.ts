import { describe, it, expect } from 'bun:test';
import {
	splitQuadletText,
	readDirectives,
	applyUnitEdits,
	joinQuadletUnits,
	readContainerCard,
	containerCardEdits,
	readPodCard,
	podCardEdits
} from './quadlet-units';

const bundle = [
	'# File: pod_stack.pod',
	'[Unit]',
	'Description=GoPod pod_stack Pod',
	'',
	'[Pod]',
	'PodName=pod_stack',
	'Network=gopod-net',
	'PublishPort=18401:80',
	'',
	'[Install]',
	'WantedBy=default.target',
	'',
	'---',
	'# File: web.container',
	'[Unit]',
	'Description=GoPod Pod Member web',
	'',
	'[Container]',
	'ContainerName=web',
	'Image=nginx:alpine',
	'Pod=pod_stack.pod',
	'Environment="API_MODE=edge"',
	'Environment="OTHER=1"',
	'Volume=web_data:/data:Z',
	'Label=io.gopod.service=proj-web-1',
	'',
	'[Service]',
	'Restart=always',
	'TimeoutStartSec=300',
	'',
	'[Install]',
	'WantedBy=default.target'
].join('\n');

describe('splitQuadletText', () => {
	it('splits a File-headered bundle into typed units', () => {
		const units = splitQuadletText(bundle);
		expect(units.map((u) => u.name)).toEqual(['pod_stack.pod', 'web.container']);
		expect(units[0].kind).toBe('pod');
		expect(units[1].kind).toBe('container');
	});

	it('parses a bare single-unit config without File header', () => {
		const units = splitQuadletText('[Unit]\n[Container]\nContainerName=only\nImage=nginx:alpine');
		expect(units).toHaveLength(1);
		expect(units[0].name).toBe('');
		expect(units[0].kind).toBe('container');
		expect(splitQuadletText('  ')).toEqual([]);
	});
});

describe('readDirectives', () => {
	it('reads all values of a repeated key with quotes stripped', () => {
		expect(readDirectives(bundle, 'Environment')).toEqual(['API_MODE=edge', 'OTHER=1']);
		expect(readDirectives(bundle, 'PublishPort')).toEqual(['18401:80']);
		expect(readDirectives('ContainerName=lower', 'containername')).toEqual(['lower']);
		expect(readDirectives('# File: x\nEnvironment="A=1"', 'Image')).toEqual([]);
	});
});

describe('applyUnitEdits', () => {
	it('replaces directive lines inside the owning section', () => {
		const web = splitQuadletText(bundle)[1];
		const next = applyUnitEdits(web.content, {
			values: { Environment: ['API_MODE=stable', 'NEW=2'], Volume: ['pgdata:/var/lib/pg:data:Z'] }
		});
		expect(readDirectives(next, 'Environment')).toEqual(['API_MODE=stable', 'NEW=2']);
		expect(readDirectives(next, 'Volume')).toEqual(['pgdata:/var/lib/pg:data:Z']);
		// untouched directives preserved
		expect(next).toContain('ContainerName=web');
		expect(next).toContain('Restart=always');
		// edits stay inside [Container]
		const containerIdx = next.indexOf('[Container]');
		const serviceIdx = next.indexOf('[Service]');
		const section = next.slice(containerIdx, serviceIdx);
		expect(section).toContain('Environment="API_MODE=stable"');
		expect(next.slice(serviceIdx)).not.toContain('Environment=');
	});

	it('empty value list removes the directive entirely', () => {
		const web = splitQuadletText(bundle)[1];
		const next = applyUnitEdits(web.content, { values: { Volume: [] } });
		expect(readDirectives(next, 'Volume')).toEqual([]);
		expect(next).toContain('ContainerName=web');
	});

	it('switches set true and remove on false', () => {
		const next = applyUnitEdits(bundle, { switches: { ReadOnly: true }, values: {} });
		expect(readDirectives(next, 'ReadOnly')).toEqual(['true']);
		const removed = applyUnitEdits(next, { switches: { ReadOnly: false }, values: {} });
		expect(readDirectives(removed, 'ReadOnly')).toEqual([]);
	});
});

describe('joinQuadletUnits roundtrip', () => {
	it('round-trips a bundle losslessly for unedited units', () => {
		const units = splitQuadletText(bundle);
		const joined = joinQuadletUnits(units);
		const reparsed = splitQuadletText(joined);
		expect(reparsed.map((u) => u.name)).toEqual(['pod_stack.pod', 'web.container']);
		expect(reparsed[0].content).toBe(units[0].content);
		expect(reparsed[1].content).toBe(units[1].content);
	});
});

describe('container card state', () => {
	it('reads env, volumes, health, and podmanArgs', () => {
		const web = splitQuadletText(bundle)[1].content;
		const state = readContainerCard(web);
		expect(state.containerName).toBe('web');
		expect(state.image).toBe('nginx:alpine');
		expect(state.env).toEqual([
			{ key: 'API_MODE', value: 'edge' },
			{ key: 'OTHER', value: '1' }
		]);
		expect(state.volumes).toEqual([{ source: 'web_data', target: '/data', options: 'Z' }]);
		expect(state.labels).toEqual([{ key: 'io.gopod.service', value: 'proj-web-1' }]);
		expect(state.readOnly).toBe(false);
	});

	it('CPU and memory ride the real PodmanArgs key with real flags', () => {
		const content = [
			'[Container]',
			'Image=nginx:alpine',
			'PodmanArgs=--share=net --cpu-quota=150000 --memory=512MB',
			'[Service]',
			'Restart=always'
		].join('\n');
		// Legacy --cpu-quota units stay readable…
		const legacy = readContainerCard(content);
		expect(legacy.cpuLimit).toBe('150000');
		expect(legacy.memoryLimit).toBe('512MB');
		// …but Apply rewrites them into the canonical --cpus form (the same
		// flag the backend generator and the runtime path emit).
		const next = containerCardEdits({ ...legacy, cpuLimit: '2', memoryLimit: '1GB' });
		const output = applyUnitEdits(content, next);
		expect(readDirectives(output, 'PodmanArgs')).toEqual(['--share=net --cpus=2 --memory=1GB']);
	});

	it('generated PodmanArgs (--cpus + --memory-swap) round-trips untouched', () => {
		const content = [
			'[Container]',
			'Image=nginx:alpine',
			'PodmanArgs=--cpus=1.50 --memory=256m --memory-swap=512MB',
			'[Service]',
			'Restart=always'
		].join('\n');
		const state = readContainerCard(content);
		expect(state.cpuLimit).toBe('1.50');
		expect(state.memoryLimit).toBe('256m');

		// Applying only a memory change must not corrupt --memory-swap into
		// "-swap=…" (the token-boundary strip bug) nor drop the swap flag.
		const next = containerCardEdits({ ...state, memoryLimit: '512MB' });
		const output = applyUnitEdits(content, next);
		expect(readDirectives(output, 'PodmanArgs')).toEqual([
			'--memory-swap=512MB --cpus=1.50 --memory=512MB'
		]);
	});

	it('full row-style edits apply as quadlet directives', () => {
		const web = splitQuadletText(bundle)[1];
		const state = readContainerCard(web.content);
		state.env = [{ key: 'API_MODE', value: 'hot reload' }];
		state.volumes = [{ source: 'db', target: '/data', options: '' }];
		state.tmpfs = ['/run:x'];
		state.capAdd = 'NET_BIND_SERVICE, SYS_TIME';
		state.capDrop = 'ALL';
		state.addDevice = '/dev/dri';
		state.healthCmd = 'curl -f http://localhost/health';
		state.healthInterval = '30s';
		state.healthOnFailure = 'kill';
		state.pidsLimit = '500';
		state.readOnly = true;
		state.readOnlyTmpfs = true;

		const next = applyUnitEdits(web.content, containerCardEdits(state));
		expect(readDirectives(next, 'Environment')).toEqual(['API_MODE=hot reload']);
		expect(readDirectives(next, 'Volume')).toEqual(['db:/data:Z']);
		expect(readDirectives(next, 'Tmpfs')).toEqual(['/run:x']);
		expect(readDirectives(next, 'AddCapability')).toEqual(['NET_BIND_SERVICE', 'SYS_TIME']);
		expect(readDirectives(next, 'DropCapability')).toEqual(['ALL']);
		expect(readDirectives(next, 'AddDevice')).toEqual(['/dev/dri']);
		expect(readDirectives(next, 'HealthCmd')).toEqual(['curl -f http://localhost/health']);
		expect(readDirectives(next, 'HealthOnFailure')).toEqual(['kill']);
		expect(readDirectives(next, 'PidsLimit')).toEqual(['500']);
		expect(readDirectives(next, 'ReadOnly')).toEqual(['true']);
		expect(readDirectives(next, 'ReadOnlyTmpfs')).toEqual(['true']);

		// the Pod= membership and labels survive
		expect(next).toContain('Pod=pod_stack.pod');
		expect(next).toContain('Label=io.gopod.service=proj-web-1');
	});
});

describe('pod card state', () => {
	it('reads and rewrites PublishPort while keeping ownership plumbing', () => {
		const podUnit = splitQuadletText(bundle)[0];
		const state = readPodCard(podUnit.content);
		expect(state.podName).toBe('pod_stack');
		expect(state.network).toEqual(['gopod-net']);
		expect(state.publishPorts).toEqual(['18401:80']);

		state.publishPorts = ['18401:80', '127.0.0.1:18402:5230/udp'];
		state.stopTimeout = '20';
		const next = applyUnitEdits(podUnit.content, podCardEdits(state));
		expect(readDirectives(next, 'PublishPort')).toEqual(['18401:80', '127.0.0.1:18402:5230/udp']);
		expect(readDirectives(next, 'StopTimeout')).toEqual(['20']);
		expect(readDirectives(next, 'Network')).toEqual(['gopod-net']);
	});
});
