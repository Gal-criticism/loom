.PHONY: dev setup build

setup:
	docker-compose up -d

dev:
	docker-compose up

build-daemon:
	cd cmd/daemon && go build -o ../../bin/loomd .

build-backend:
	bun run backend/src/index.ts

clean:
	docker-compose down -v
