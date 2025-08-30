APP=go-aws-eks-get-token

.PHONY: build lint clean

build:
	go build -o $(APP) main.go

lint:
	golangci-lint run

clean:
	rm -f $(APP)
