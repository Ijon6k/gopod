export interface QuadletTemplate {
	id: string;
	name: string;
	description?: string;
	image: string;
	port: number;
	content: string;
}

export const QUADLET_TEMPLATES: QuadletTemplate[] = [
	{
		id: 'metube',
		name: 'MeTube (YouTube Downloader)',
		description: 'Web GUI for youtube-dl / yt-dlp with auto-downloading',
		image: 'ghcr.io/alexta69/metube:latest',
		port: 8081,
		content: `[Unit]
Description=MeTube Video Downloader Quadlet
After=network-online.target

[Container]
Image=ghcr.io/alexta69/metube:latest
PublishPort=8081:8081
Volume=metube-downloads:/downloads:Z
AutoUpdate=registry
Restart=always

[Service]
Restart=always
TimeoutStartSec=300

[Install]
WantedBy=default.target`
	},
	{
		id: 'nginx',
		name: 'Nginx Web Server',
		description: 'High performance HTTP and reverse proxy server',
		image: 'docker.io/library/nginx:alpine',
		port: 8080,
		content: `[Unit]
Description=Nginx Web Server Quadlet
After=network-online.target

[Container]
Image=docker.io/library/nginx:alpine
PublishPort=8080:80
AutoUpdate=registry
Restart=always

[Service]
Restart=always

[Install]
WantedBy=default.target`
	},
	{
		id: 'postgres',
		name: 'PostgreSQL 17 Database',
		description: 'Reliable relational database management system',
		image: 'docker.io/library/postgres:17-alpine',
		port: 5432,
		content: `[Unit]
Description=PostgreSQL Database Quadlet
After=network-online.target

[Container]
Image=docker.io/library/postgres:17-alpine
PublishPort=5432:5432
Environment=POSTGRES_PASSWORD=postgres_secure_pass
Volume=postgres-data:/var/lib/postgresql/data:Z
Restart=always

[Service]
Restart=always
TimeoutStartSec=600

[Install]
WantedBy=default.target`
	},
	{
		id: 'redis',
		name: 'Redis In-Memory Cache',
		description: 'Key-value cache and real-time message broker',
		image: 'docker.io/library/redis:7-alpine',
		port: 6379,
		content: `[Unit]
Description=Redis In-Memory Cache Quadlet
After=network-online.target

[Container]
Image=docker.io/library/redis:7-alpine
PublishPort=6379:6379
Restart=always

[Service]
Restart=always

[Install]
WantedBy=default.target`
	},
	{
		id: 'caddy',
		name: 'Caddy Web Server',
		description: 'Modern HTTP/2 & HTTP/3 server with automatic HTTPS',
		image: 'docker.io/library/caddy:alpine',
		port: 80,
		content: `[Unit]
Description=Caddy Web Server Quadlet
After=network-online.target

[Container]
Image=docker.io/library/caddy:alpine
PublishPort=80:80
PublishPort=443:443
Volume=caddy-data:/data:Z
Volume=caddy-config:/config:Z
AutoUpdate=registry
Restart=always

[Service]
Restart=always

[Install]
WantedBy=default.target`
	}
];
