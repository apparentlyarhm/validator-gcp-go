BINARY_NAME=validator
run: 
	go run main.go

build:
	go build -o ${BINARY_NAME} .

build-prod:
	go build -ldflags="-s -w" -o ${BINARY_NAME} .

clean:
	go clean
	rm ${BINARY_NAME}