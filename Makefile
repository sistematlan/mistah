build:
	go build -o bin/chipawa .

run:
	go run . $(ARGS)

test:
	go test ./...

clean-bin:
	rm -rf bin/
