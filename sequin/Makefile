sequin-latest:
	curl -L https://github.com/sequinstream/sequin/releases/latest/download/sequin-docker-compose.zip -o sequin-docker-compose.zip \
	&& unzip sequin-docker-compose.zip -d docker && rm sequin-docker-compose.zip

sequin:
	cd docker/sequin-docker-compose && docker compose up -d