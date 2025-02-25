# UFinder

UFinder is a powerful Go-based URL discovery and aggregation tool designed for security researchers and bug bounty hunters. It combines and orchestrates multiple URL discovery tools to find web endpoints efficiently, eliminate duplicates, and provide a comprehensive view of a target's attack surface.

## Features

- **Multi-tool Orchestration**: Run multiple URL discovery tools concurrently with a single command
- **Automatic Deduplication**: Filter and maintain unique URL collections
- **Incremental Discovery**: Track new URLs discovered across multiple scans
- **Comparative Analysis**: Compare the effectiveness of different URL discovery tools
- **Organized Output**: Results are saved in a structured directory format

## Supported Tools

UFinder integrates with the following popular URL discovery tools:

- **Waymore**: Discover URLs from the Wayback Machine
- **Waybackurls**: Extract URLs from the Wayback Machine archive
- **GAU**: Get All URLs from various sources
- **XURLFinder**: Advanced URL discovery with subdomain support
- **URLScan**: Retrieve URLs from the URLScan.io API
- **URLFinder**: Find URLs using custom patterns
- **Ducker**: Extract URLs from search engine results

## Prerequisites

Before using UFinder, ensure you have the following tools installed:

- Go 1.16+
- The integrated tools (waymore, waybackurls, gau, xurlfind3r, urlfinder)
- Python 3.x (for Ducker)
- jq (for URLScan results processing)

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/ufinder.git
cd ufinder

# Build the binary
go build -o ufinder

# Make it executable and move to your PATH
chmod +x ufinder
sudo mv ufinder /usr/local/bin/
```

## Usage

Basic usage:

```bash
ufinder -d example.com -f output_directory
```

### Command Line Options

| Flag | Description |
|------|-------------|
| `-d` | Target domain (required unless using `-c`) |
| `-f` | Output folder name (required) |
| `-c` | Compare unique URLs found per tool |
| `-t` | Run specific tool(s), comma-separated (e.g., waymore,urlfinder) |

### Examples

Run all tools against a domain:
```bash
ufinder -d example.com -f example_recon
```

Run specific tools:
```bash
ufinder -d example.com -f example_recon -t waymore,gau,urlscan
```

Compare the effectiveness of previously run tools:
```bash
ufinder -f example_recon -c
```

## Output Structure

```
output_directory/
└── endpoints/
    ├── urls.txt                # Master file with all unique URLs
    ├── waymore.txt             # URLs found by waymore
    ├── waybackurls.txt         # URLs found by waybackurls
    ├── gau.txt                 # URLs found by gau
    ├── xurlfind3r.txt          # URLs found by xurlfind3r
    ├── urlscan.txt             # URLs found by urlscan
    ├── urlfinder.txt           # URLs found by urlfinder
    ├── ducker.txt              # URLs found by ducker
    ├── last_waymore.txt        # New URLs found in the latest waymore run
    ├── last_waybackurls.txt    # New URLs found in the latest waybackurls run
    └── ...                     # Similar files for other tools
```

## Environment Variables

For URLScan.io integration, set your API key:

```bash
export URLSCAN="your_urlscan_api_key"
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Created by Gilson Oliveira
- Thanks to the developers of all integrated tools
