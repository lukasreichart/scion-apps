# Path Negotiation Example

This is an example how to use the path negotiation protocol implemented in `pkg/pathneg/`.

## Setup
To run the example you should have SCION installed on the machine, either by using the
[SCION development setup](https://scion.docs.anapaya.net/en/latest/build/setup.html)
or using [SCION lab](https://docs.scionlab.org/).

## Run the example

To start the path negotiation server, you can run:
````bash
go run example.go -server
````

to run the client:
```bash
go run example.go
```

## Arguments
The arguments for the `example.go` program are:

| Argument | Description | Example Value |
| -------- | ----------- | ------------- |
| `-server` | if set this will start the server instead of the client | `-server` |
| `-address` | the address of the host to perform path negotiation with | `-address=17-ffaa:1:e8b,[127.0.0.1]:2222`|
