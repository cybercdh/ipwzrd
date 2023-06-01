# ipwzrd

`ipwzrd` is a Go program that takes a list of domain names piped from stdin, looks up the A record for each domain, and checks if the IP address is dead. If the IP address is dead, the program prints it. Additionally, if the IP address belongs to an EC2 host, the program highlights it. 


## Installation

The installation requires you to already have Go installed. Assuming this, then run:

```bash
go install github.com/cybercdh/ipwzrd@latest
```

## Usage

```bash
cat <domains> | ipwzrd
````
or
```bash
ipwzrd example.com
```

Options:
```
Usage of ipwzrd:
  -c int
    	set the concurrency level (default 20)
```

## Contributing

Pull requests are welcome. For major changes, please open an issue first
to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License

[MIT](https://choosealicense.com/licenses/mit/)