import { describe, it, expect } from 'bun:test';
import {
	parseCpuLimit,
	parseMemoryMb,
	parsePidsLimit,
	normalizeSwapLimit,
	parseCapList,
	validateVolumeRow,
	validateUserNamespace,
	validateDeviceList
} from './advanced-validation';

describe('parseCpuLimit', () => {
	it('empty means unlimited (0), numbers are decimals in a sane bound', () => {
		expect(parseCpuLimit('')).toBe(0);
		expect(parseCpuLimit('   ')).toBe(0);
		expect(parseCpuLimit('2')).toBe(2);
		expect(parseCpuLimit('1.5')).toBe(1.5);
		expect(parseCpuLimit('64')).toBe(64);
	});

	it('rejects garbage and out-of-range values without coercion', () => {
		expect(parseCpuLimit('abc')).toBeNaN();
		expect(parseCpuLimit('-1')).toBeNaN();
		expect(parseCpuLimit('0')).toBeNaN();
		expect(parseCpuLimit('65')).toBeNaN();
		expect(parseCpuLimit('2x')).toBeNaN();
	});
});

describe('parseMemoryMb', () => {
	it('empty means unlimited (0); units translate to MB', () => {
		expect(parseMemoryMb('')).toBe(0);
		expect(parseMemoryMb('512')).toBe(512);
		expect(parseMemoryMb('512MB')).toBe(512);
		expect(parseMemoryMb('512 mb')).toBe(512);
		expect(parseMemoryMb('1G')).toBe(1024);
		expect(parseMemoryMb('2.5GB')).toBe(2560);
	});

	it('rejects garbage instead of inventing a default', () => {
		expect(parseMemoryMb('big')).toBeNaN();
		expect(parseMemoryMb('512x')).toBeNaN();
		expect(parseMemoryMb('0.5KB')).toBeNaN();
		expect(parseMemoryMb('0MB')).toBeNaN();
	});
});

describe('parsePidsLimit', () => {
	it('empty means unlimited (0)', () => {
		expect(parsePidsLimit('')).toBe(0);
		expect(parsePidsLimit('500')).toBe(500);
	});

	it('rejects non-integers without fabricating 2048', () => {
		expect(parsePidsLimit('abc')).toBeNaN();
		expect(parsePidsLimit('12.5')).toBeNaN();
		expect(parsePidsLimit('-3')).toBeNaN();
	});
});

describe('normalizeSwapLimit', () => {
	it('mirrors the backend podman.NormalizeMemorySwap grammar', () => {
		expect(normalizeSwapLimit('')).toEqual({ value: '', error: '' });
		expect(normalizeSwapLimit('0')).toEqual({ value: '', error: '' });
		expect(normalizeSwapLimit('default')).toEqual({ value: '', error: '' });
		expect(normalizeSwapLimit('unlimited')).toEqual({ value: '-1', error: '' });
		expect(normalizeSwapLimit('UNLIMITED')).toEqual({ value: '-1', error: '' });
		expect(normalizeSwapLimit('-1')).toEqual({ value: '-1', error: '' });
		expect(normalizeSwapLimit('512MB')).toEqual({ value: '512MB', error: '' });
		expect(normalizeSwapLimit('512 MB')).toEqual({ value: '512MB', error: '' });
		expect(normalizeSwapLimit('1g')).toEqual({ value: '1g', error: '' });
		expect(normalizeSwapLimit('262144000')).toEqual({ value: '262144000', error: '' });
	});

	it('reports specific errors instead of passing them through', () => {
		expect(normalizeSwapLimit('garbage!').error).toContain('invalid memory swap value');
		expect(normalizeSwapLimit('MB').error).toContain('invalid memory swap value');
		expect(normalizeSwapLimit('12x').error).toContain('use a size like');
	});
});

describe('parseCapList', () => {
	it('splits and trims valid capability tokens', () => {
		expect(parseCapList('NET_BIND_SERVICE, SYS_TIME')).toEqual(['NET_BIND_SERVICE', 'SYS_TIME']);
		expect(parseCapList('')).toEqual([]);
	});

	it('null on format garbage so the UI blocks save', () => {
		expect(parseCapList('NET_BIND;SERVICE')).toBe(null);
		expect(parseCapList('all caps')).toBe(null);
	});
});

describe('validateVolumeRow', () => {
	it('accepts named volumes and absolute paths with option tokens', () => {
		expect(validateVolumeRow({ source: 'app_data', target: '/app/data', options: 'Z' })).toBe('');
		expect(validateVolumeRow({ source: '/srv/media', target: '/media', options: 'ro,Z' })).toBe('');
		expect(
			validateVolumeRow({ source: '/srv/a', target: '/b', options: 'uid=1000,gid=1000,ro' })
		).toBe('');
	});

	it('explains exactly what is wrong', () => {
		expect(validateVolumeRow({ source: '', target: '/b', options: '' })).toContain('Source is');
		expect(validateVolumeRow({ source: 'rel/path', target: '/b', options: '' })).toContain(
			'absolute host path or a named volume'
		);
		expect(validateVolumeRow({ source: 'app_data', target: 'app/data', options: '' })).toContain(
			'absolute container path'
		);
		expect(validateVolumeRow({ source: 'app_data', target: '/b', options: 'ro;$()' })).toContain(
			'Invalid mount option'
		);
	});
});

describe('validateUserNamespace', () => {
	it('empty is fine; podman tokens pass', () => {
		expect(validateUserNamespace('')).toBe('');
		expect(validateUserNamespace('keep-id')).toBe('');
		expect(validateUserNamespace('auto:size=1024:512')).toBe('');
		expect(validateUserNamespace('host')).toBe('');
	});

	it('shell characters are rejected before a deploy can fail', () => {
		expect(validateUserNamespace('keep id')).toContain('Namespace values');
		expect(validateUserNamespace('size=$(rm)')).toContain('Namespace values');
	});
});

describe('validateDeviceList', () => {
	it('accepts /dev paths and explains anything else', () => {
		expect(validateDeviceList('/dev/fuse, /dev/net/tun')).toBe('');
		expect(validateDeviceList('fuse')).toContain('under /dev/');
		expect(validateDeviceList('')).toBe('');
	});
});
