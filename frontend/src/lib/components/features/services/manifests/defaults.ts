export function getDefaultQuadletConfig(service: {
	name: string;
	image?: string;
	port?: number;
}): string {
	return `[Unit]
Description=${service.name} Quadlet Service
After=network-online.target

[Container]
Image=${service.image || 'docker.io/library/nginx:alpine'}
PublishPort=${service.port || 8080}:80
AutoUpdate=registry
Restart=always

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
