default:
	go build -o ./build .

windows:
	GOOS=windows go build -o ./build .
linux:
	GOOS=linux go build -o ./build .
all: windows linux