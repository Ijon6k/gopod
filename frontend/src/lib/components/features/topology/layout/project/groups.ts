// ──────────────────────────────────────────────
// GOPOD Topology — Workload Group Builder
// One runtime pod = exactly one group. Member services never render
// their own pod cards; their containers live inside the pod group.
// ──────────────────────────────────────────────

import type { Service, Container } from '$lib/types';
import type { TopologyNode } from '../../types';
import { podServiceNames } from '../../pod-naming';
import { isPodMemberService, isInfraContainer, ownerServiceId, type PodAssignment } from './ownership';

export interface WorkloadItem {
	id: string;
	name: string;
	image: string;
	status: string;
	cpu?: number;
	memory?: number;
	infra?: boolean;
	serviceId: string;
	raw: any;
}

export interface GroupBuilderContext {
	projectId: string;
	nodes: TopologyNode[];
	containerIdMap: Map<string, string>;
	serviceToFirstContainerMap: Map<string, string>;
	serviceGroupMap: Map<string, TopologyNode>;
	serviceYMap: Map<string, { y: number; height: number; centerY: number }>;
	currentWorkloadY: number;
	groupWidth: number;
	childWidth: number;
	childHeight: number;
	childGap: number;
	groupPaddingTop: number;
	groupPaddingBottom: number;
	colWorkloadsX: number;
}

export type GroupType = 'pod' | 'compose' | 'quadlet' | 'service';

export function toWorkloadItem(c: Container, ownerId: string, infra: boolean): WorkloadItem {
	return {
		id: `container-${c.id}`,
		name: c.name,
		image: c.image.split('/').pop() || (infra ? 'infra:pause' : c.image) || 'container:latest',
		status: c.status,
		cpu: c.cpu,
		memory: c.memory,
		infra,
		serviceId: ownerId,
		raw: c
	};
}

export function pushGroup(
	ctx: GroupBuilderContext,
	opts: {
		id: string;
		title: string;
		status: string;
		groupType: GroupType;
		badge: string;
		serviceId: string;
		monoDetail: string;
		items: WorkloadItem[];
		raw: any;
	}
): TopologyNode {
	const childCount = opts.items.length;
	const groupHeight = Math.max(
		105,
		ctx.groupPaddingTop +
			childCount * (ctx.childHeight + ctx.childGap) -
			ctx.childGap +
			ctx.groupPaddingBottom
	);
	const totalCpu = opts.items.reduce((acc, curr) => acc + (curr.cpu || 0), 0);
	const groupNode: TopologyNode = {
		id: opts.id,
		type: 'group',
		isGroup: true,
		groupType: opts.groupType,
		title: opts.title,
		subtitle: `${childCount} container${childCount > 1 ? 's' : ''} · ${totalCpu.toFixed(1)}% CPU`,
		status: opts.status,
		x: ctx.colWorkloadsX,
		y: ctx.currentWorkloadY,
		width: ctx.groupWidth,
		height: groupHeight,
		monoDetail: opts.monoDetail,
		badge: opts.badge,
		projectId: ctx.projectId,
		serviceId: opts.serviceId,
		raw: opts.raw
	};
	ctx.nodes.push(groupNode);
	ctx.serviceGroupMap.set(opts.serviceId || opts.id, groupNode);
	const svcCenterY = ctx.currentWorkloadY + Math.min(groupHeight / 2, 45);
	ctx.serviceYMap.set(opts.serviceId || opts.id, {
		y: ctx.currentWorkloadY,
		height: groupHeight,
		centerY: svcCenterY
	});
	opts.items.forEach((w, idx) => {
		const childNode: TopologyNode = {
			id: w.id,
			type: 'container',
			parentId: groupNode.id,
			title: w.name,
			subtitle: w.image,
			status: w.status,
			x: 16,
			y: ctx.groupPaddingTop + idx * (ctx.childHeight + ctx.childGap),
			width: ctx.childWidth,
			height: ctx.childHeight,
			monoDetail: `${(w.cpu || 0).toFixed(1)}% · ${w.memory || 0}MB`,
			badge:
				opts.groupType === 'compose'
					? 'Compose'
					: opts.groupType === 'quadlet'
						? 'Quadlet'
						: opts.groupType === 'pod'
							? w.infra
								? 'Pod Infra'
								: 'Pod Member'
							: 'Container',
			projectId: ctx.projectId,
			serviceId: w.serviceId,
			raw: w.raw
		};
		ctx.nodes.push(childNode);
		ctx.containerIdMap.set(w.name, childNode.id);
		ctx.containerIdMap.set(w.id, childNode.id);
		if (w.raw && typeof w.raw.id === 'string' && w.raw.id) {
			ctx.containerIdMap.set(w.raw.id, childNode.id);
		}
		if (idx === 0) {
			ctx.serviceToFirstContainerMap.set(opts.serviceId || opts.id, childNode.id);
		}
	});
	// 24px spacing between groups (comfortable, not spread out)
	ctx.currentWorkloadY += groupHeight + 24;
	return groupNode;
}

// Point member services at their pod group so domain/port edges always
// have a visible cable target even when a member has no live container.
export function anchorMembersToGroup(
	ctx: GroupBuilderContext,
	groupNode: TopologyNode,
	memberServiceIds: string[]
): void {
	for (const id of memberServiceIds) {
		if (id && !ctx.serviceGroupMap.has(id)) {
			ctx.serviceGroupMap.set(id, groupNode);
		}
	}
}

export function buildPodGroups(
	ctx: GroupBuilderContext,
	args: {
		podServices: Service[];
		allPods: { name: string; status?: string }[];
		projContainers: Container[];
		projServices: Service[];
		projServiceIds: Set<string>;
		containerAssignment: Map<string, PodAssignment>;
		assignedContainerIds: Set<string>;
	}
): Set<string> {
	const renderedPodNames = new Set<string>();
	for (const svc of args.podServices) {
		const names = podServiceNames(svc);
		const rtPod = args.allPods.find((p) => names.includes(p.name));
		const podName = rtPod ? rtPod.name : names[0];
		if (renderedPodNames.has(podName)) continue;
		renderedPodNames.add(podName);

		const items: WorkloadItem[] = [];
		for (const c of args.projContainers) {
			const a = args.containerAssignment.get(c.id);
			if (a && a.kind === 'pod' && a.podName === podName) {
				const ownerId = ownerServiceId(c, args.projServices, args.projServiceIds) || svc.id;
				items.push(toWorkloadItem(c, ownerId, isInfraContainer(c)));
				args.assignedContainerIds.add(c.id);
				if (!ctx.serviceToFirstContainerMap.has(ownerId)) {
					ctx.serviceToFirstContainerMap.set(ownerId, `container-${c.id}`);
				}
				if (!ctx.serviceYMap.has(ownerId)) {
					ctx.serviceYMap.set(ownerId, {
						y: ctx.currentWorkloadY,
						height: 105,
						centerY: ctx.currentWorkloadY + 45
					});
				}
			}
		}
		items.sort((a, b) => (a.infra === b.infra ? 0 : a.infra ? -1 : 1));

		const transitional =
			svc.status === 'deploying' ||
			svc.status === 'building' ||
			svc.status === 'starting';
		if (items.length === 0) {
			items.push({
				id: `workload-${svc.id}-pending`,
				name: svc.name,
				image: 'container:latest',
				status: transitional ? svc.status : 'stopped',
				serviceId: svc.id,
				raw: svc
			});
		}
		pushGroup(ctx, {
			id: `group-${svc.id}`,
			title: rtPod ? rtPod.name : svc.name,
			status: rtPod ? svc.status : transitional ? svc.status : 'stopped',
			groupType: 'pod',
			badge: 'Podman Pod',
			serviceId: svc.id,
			monoDetail: svc.port ? `:${svc.port}` : '',
			items,
			raw: svc
		});
		const podGroupNode = ctx.serviceGroupMap.get(svc.id);
		if (podGroupNode) {
			const memberIds: string[] = [];
			for (const c of args.projContainers) {
				const a = args.containerAssignment.get(c.id);
				if (a && a.kind === 'pod' && a.podName === podName) {
					const ownerId = ownerServiceId(c, args.projServices, args.projServiceIds) || svc.id;
					if (ownerId !== svc.id) memberIds.push(ownerId);
				}
			}
			// Anchor every member service by PodID too, even ones with no
			// live container, so domain cables always land on the pod group.
			const names = podServiceNames(svc);
			for (const m of args.projServices) {
				if (m.id !== svc.id && m.podId && names.includes(m.podId)) {
					memberIds.push(m.id);
				}
			}
			anchorMembersToGroup(ctx, podGroupNode, memberIds);
		}
		// Anchor member services to the real pod geometry for domain edges.
		const g = ctx.serviceYMap.get(svc.id);
		if (g) {
			for (const c of args.projContainers) {
				const a = args.containerAssignment.get(c.id);
				if (a && a.kind === 'pod' && a.podName === podName) {
					ctx.serviceYMap.set(ownerServiceId(c, args.projServices, args.projServiceIds) || svc.id, g);
				}
			}
		}
	}
	return renderedPodNames;
}

export function buildOrphanPodGroups(
	ctx: GroupBuilderContext,
	args: {
		allPods: { name: string; status?: string }[];
		projContainers: Container[];
		projServices: Service[];
		projServiceIds: Set<string>;
		projectId: string;
		containerAssignment: Map<string, PodAssignment>;
		assignedContainerIds: Set<string>;
		renderedPodNames: Set<string>;
	}
): void {
	for (const p of args.allPods) {
		if (args.renderedPodNames.has(p.name)) continue;
		const members = args.projContainers.filter((c) => {
			const a = args.containerAssignment.get(c.id);
			return !!a && a.kind === 'pod' && a.podName === p.name;
		});
		if (members.length === 0) continue;
		// Only claim pods touching this project; foreign pods stay invisible.
		const touches = members.some((c) => {
			const ownerId = ownerServiceId(c, args.projServices, args.projServiceIds);
			return (
				(ownerId && args.projServiceIds.has(ownerId)) ||
				c.labels?.['io.gopod.project'] === args.projectId ||
				c.projectId === args.projectId
			);
		});
		if (!touches) continue;
		args.renderedPodNames.add(p.name);
		const items = members.map((c) =>
			toWorkloadItem(c, ownerServiceId(c, args.projServices, args.projServiceIds) || p.name, isInfraContainer(c))
		);
		items.sort((a, b) => (a.infra === b.infra ? 0 : a.infra ? -1 : 1));
		for (const c of members) args.assignedContainerIds.add(c.id);
		pushGroup(ctx, {
			id: `group-orphan-${p.name}`,
			title: p.name,
			status: (p.status || '').toLowerCase() === 'running' ? 'running' : 'stopped',
			groupType: 'pod',
			badge: 'Podman Pod',
			serviceId: p.name,
			monoDetail: '',
			items,
			raw: p
		});
	}
}

export function buildServiceGroups(
	ctx: GroupBuilderContext,
	args: {
		projServices: Service[];
		projContainers: Container[];
		projServiceIds: Set<string>;
		assignedContainerIds: Set<string>;
	}
): void {
	const unassigned = (c: Container) => !args.assignedContainerIds.has(c.id);
	for (const svc of args.projServices) {
		if (svc.type === 'pod' || svc.type === 'kubernetes') continue;
		const owned = args.projContainers.filter(
			(c) => unassigned(c) && ownerServiceId(c, args.projServices, args.projServiceIds) === svc.id
		);

		const workloadItems: WorkloadItem[] = [];
		if (owned.length > 0) {
			owned.forEach((c) => {
				workloadItems.push(toWorkloadItem(c, svc.id, false));
				args.assignedContainerIds.add(c.id);
			});
		} else {
			// Live containers exist but all placed elsewhere (e.g. inside a
			// pod group): render nothing instead of a phantom card.
			const hasLiveElsewhere = args.projContainers.some(
				(c) => ownerServiceId(c, args.projServices, args.projServiceIds) === svc.id
			);
			if (hasLiveElsewhere) continue;
			if (svc.workloads && svc.workloads.length > 0 && !isPodMemberService(svc)) {
				svc.workloads.forEach((wl: any) => {
					workloadItems.push({
						id: `workload-${svc.id}-${wl.name}`,
						name: wl.name,
						image: wl.image || 'container:latest',
						status: wl.status || svc.status,
						cpu: wl.cpu,
						memory: wl.memory,
						serviceId: svc.id,
						raw: wl
					});
				});
			} else if (!isPodMemberService(svc)) {
				// Single container fallback. Member services with zero live
				// containers render nothing: they live inside their pod group.
				workloadItems.push({
					id: `container-${svc.id}`,
					name: svc.type === 'quadlet' ? `${svc.name}.service` : svc.name,
					image: svc.image ? svc.image.split('/').pop() || svc.image : 'container:latest',
					status: svc.status,
					cpu: svc.cpu,
					memory: svc.memory,
					serviceId: svc.id,
					raw: svc
				});
			}
		}
		if (workloadItems.length === 0) continue;

		const groupType = svc.type === 'compose' ? 'compose' : svc.type === 'quadlet' ? 'quadlet' : 'service';
		pushGroup(ctx, {
			id: `group-${svc.id}`,
			title: svc.name,
			status: svc.status,
			groupType,
			badge:
				groupType === 'compose'
					? 'Compose Stack'
					: groupType === 'quadlet'
						? 'Quadlet Unit'
						: 'Service',
			serviceId: svc.id,
			monoDetail: svc.port ? `:${svc.port}` : '',
			items: workloadItems,
			raw: svc
		});
	}
}

export function buildUnmanagedGroup(
	ctx: GroupBuilderContext,
	projContainers: Container[],
	assignedContainerIds: Set<string>
): void {
	const orphans = projContainers.filter((c) => !assignedContainerIds.has(c.id));
	if (orphans.length === 0) return;
	pushGroup(ctx, {
		id: `group-unmanaged`,
		title: 'Unmanaged',
		status: 'degraded',
		groupType: 'service',
		badge: 'Unmanaged',
		serviceId: 'unmanaged',
		monoDetail: '',
		items: orphans.map((c) => toWorkloadItem(c, 'unmanaged', false)),
		raw: null
	});
	for (const c of orphans) assignedContainerIds.add(c.id);
}
