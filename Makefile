
build:
	echo ">> build binaries..."
	sh -c ./scripts/build.sh

docker:
	echo ">> build docker image..."
	docker build -t melonproject/ethereum-exporter .

publish:
	echo ">> publish docker image..."
	docker push melonproject/ethereum-exporter:latest

.PHONY: build docker
