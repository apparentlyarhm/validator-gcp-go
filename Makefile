BINARY_NAME=validator
run: 
	go run main.go

build:
	go build -o out/${BINARY_NAME} ./main.go && ls -lh out/${BINARY_NAME}

build-prod:
	go build -ldflags="-s -w" -o out/${BINARY_NAME} ./main.go && ls -lh out/${BINARY_NAME}

clean:
	go clean
	rm out/${BINARY_NAME}