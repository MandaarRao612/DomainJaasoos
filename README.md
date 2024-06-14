Here's an edited version for your `README.md` file that includes a notice about ethical use:

---

# Domain Jasoos

**Domain Jasoos** is a specialized tool for probing subdomains and segregating them based on their HTTP status codes. It provides detailed output in JSON format, categorizing subdomains and showing redirection details where applicable.

## Features

- Probes subdomains for HTTP and HTTPS.
- Categorizes based on HTTP status codes.
- Records redirection details for 3xx status codes.
- Outputs results in a structured JSON file.

## Installation

Clone the repository and build the tool using `go build`:

```bash
git clone https://github.com/your-username/domain-jasoos.git
cd domain-jasoos
go build -o domain-jasoos
```

## Usage

After building, you can run `domain-jasoos` and pipe in a list of subdomains:

```bash
cat subdomains.txt | ./domain-jasoos
```

The output will be a JSON file named with the current date and time, saved in the current directory. It will look something like this:

```json
{
    "200": [
        "https://onthego.tatamotors.com",
        "https://acetest.tatamotors.com"
    ],
    "301": [
        {
            "url": "http://www.buses.tatamotors.com",
            "redirected_url": "https://buses.tatamotors.com"
        }
    ],
    "302": [
        {
            "url": "http://eworkshop.tatamotors.com",
            "redirected_url": "https://eworkshop-redirect.tatamotors.com"
        }
    ],
    "403": [
        "https://middleeast.countrysites.tatamotors.com"
    ]
}
```

## Ethical Use

**Domain Jasoos** is a tool for ethical use. Please ensure you have proper authorization before probing any domains. Unauthorized use of this tool on domains you do not own or have explicit permission to test could be illegal and unethical.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Feel free to submit issues or pull requests. Contributions are welcome!

---

You can paste this content directly into your `README.md` file in your repository.
