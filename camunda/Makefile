sequin:
	curl -L https://github.com/sequinstream/sequin/releases/latest/download/sequin-docker-compose.zip -o sequin-docker-compose.zip \
	&& unzip sequin-docker-compose.zip -d docker && rm sequin-docker-compose.zip
	
camunda:
	cd docker/camunda && docker compose up -d

dbmate-status:
	docker run --rm --network camunda_default \
		-v $(PWD)/docker/camunda/migrations:/db/migrations \
		amacneil/dbmate:2.15.0 \
		--url "postgres://camunda:camunda@camunda_postgres:5432/process-engine?sslmode=disable" \
		status

dbmate-apply:
	docker run --rm --network camunda_default \
		-v $(PWD)/docker/camunda/migrations:/db/migrations \
		amacneil/dbmate:2.15.0 \
		--url "postgres://camunda:camunda@camunda_postgres:5432/process-engine?sslmode=disable" \
		--no-dump-schema up