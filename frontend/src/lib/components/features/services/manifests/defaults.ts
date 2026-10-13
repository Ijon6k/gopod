export function getDefaultQuadletConfig(service: {
	name: string;
	image?: string;
	port?: number;
	hostPort?: number;
}): string {
	const portLine =
		service.hostPort && service.hostPort > 0
			? `PublishPort=${service.hostPort}:${service.port || service.hostPort}\n`
			: '';
	return `[Unit]
Description=${service.name} Quadlet Service
After=network-online.target
StartLimitIntervalSec=60s
StartLimitBurst=5

[Container]
Image=${service.image || 'docker.io/library/nginx:alpine'}
Network=gopod-net
${portLine}AutoUpdate=registry

[Service]
Restart=always
TimeoutStartSec=300

[Install]
WantedBy=default.target`;
}

export function getDefaultComposeYaml(service: {
	name: string;
	image?: string;
	port?: number;
}): string {
	return `version: "3.8"
services:
  ${service.name}:
    image: ${service.image || 'nginx:alpine'}
    restart: always
    ports:
      - "${service.port || 8080}:80"
    environment:
      - NODE_ENV=production`;
}
