# Persysctl Documentation

## Overview

The `persysctl` is a command-line interface (CLI) tool for interacting with the Persys system(s), enabling users to manage workloads, nodes, and cluster metrics. Built with Go, it uses the Cobra framework for command-line parsing and Viper for configuration management. The CLI communicates with the Prow system via HTTP API endpoints, allowing users to schedule workloads, list workloads and nodes, and retrieve system metrics.
This documentation covers installation, initial configuration, command usage, and troubleshooting for the persysctl.

## Prerequisites

Before using the persysctl, ensure the following:

* Operating System: Linux, macOS, or Windows (with Go installed).
* Go: Version 1.16 or higher.
* Network Access: Access to the Prow system’s API endpoint (e.g., `<http://localhost:8084>`).
* Configuration: A valid configuration file (default: `~/.persys/config.yaml`) with the API endpoint.

## Installation

1. Clone the Repository (if available):

```sh
git clone https://github.com/persys-dev/persysctl.git
cd persysctl
```

2. Install persysctl using make:

```sh
make install
```

3. Verify Installation:

```sh
persysctl --help
```

Note: The provided code doesn’t implement a --version flag.

## Initial Configuration

The `persysctl` requires a configuration file to specify the Prow system’s API endpoint and other settings. By default, it looks for `~/.persys/config.yaml`. The configuration is managed using Viper, which supports YAML format and environment variable overrides.

**Step 1: Create the Configuration Directory**
Create the .persys directory in your home directory:

```sh
mkdir -p ~/.persys
```

**Step 2: Create the Configuration File**
Create a config.yaml file at `~/.persys/config.yaml` with the following structure:

```yaml
api_endpoint: <http://localhost:8080>
verbose: false
```

* api_endpoint: The URL of the Prow system’s API (required).
* verbose: Enables verbose logging (optional, defaults to false).

Example:

```sh
echo "api_endpoint: <http://192.168.1.100:8084>" > ~/.persys/config.yaml
```

Alternatively, specify a custom config file using the `--config` flag:

```sh
persysctl --config /path/to/custom-config.yaml
```

**Step 3: Set Environment Variables (Optional)**
You can override configuration settings using environment variables. Viper maps environment variables by prefixing keys with `PERSYS_`(case-insensitive). For example:

```sh
export PERSYS_API_ENDPOINT=<http://192.168.1.100:8080>
export PERSYS_VERBOSE=true
```

**Step 4: Verify Configuration**
Run a command to ensure the CLI can read the configuration:

```sh
persysctl nodes --verbose
```

If the configuration is invalid or missing, you’ll see an error. Check `~/.persys/config.yaml` and ensure the API endpoint is accessible.

**Step 5: Secure the Configuration**
Restrict permissions to protect sensitive data (e.g., API credentials, if added):

```sh
chmod 600 ~/.persys/config.yaml
```

## Connecting to a prow scheduler server with mTLS

This CLI uses mutual TLS (mTLS) to securely connect to the prow scheduler server. To connect successfully, follow these steps:

### 1. Ensure Server Certificate SANs
- The prow scheduler server **must** present a TLS certificate with the correct Subject Alternative Names (SANs).
- The SANs must include the DNS name or IP address you use to connect (e.g., `localhost`, `127.0.0.1`, or your server's IP).
- If you see errors like:
  - `x509: cannot validate certificate for <ip> because it doesn't contain any IP SANs`
  - `x509: certificate is not valid for any names, but wanted to match <hostname>`
  This means the server certificate is missing the required SANs.

### 2. Configure the Client
- Set the `APIEndpoint` in your config to the exact DNS name or IP that matches a SAN in the server's certificate.
- Example:
  ```yaml
  apiEndpoint: https://localhost:8085
  # or
  apiEndpoint: https://192.168.1.13:8085
  ```
- Ensure your client has access to the CA certificate that signed the server's certificate.

### 3. Troubleshooting
- If you cannot change the server certificate and are only testing locally, you can set the client to skip certificate verification (INSECURE):
  - In the code, set `InsecureSkipVerify: true` in the TLS config (see `internal/auth/cert.go`).
  - **Warning:** This disables all security checks and should never be used in production.

### 4. Regenerating Certificates
- If you control the server, regenerate its certificate to include all required SANs (DNS names and IPs you will use to connect).
- Restart the server with the new certificate.

### 5. Example Error Messages and Solutions
| Error Message                                                                 | Solution                                      |
|-------------------------------------------------------------------------------|-----------------------------------------------|
| `x509: cannot validate certificate for <ip> because it doesn't contain any IP SANs` | Add the IP to the server certificate's SANs   |
| `x509: certificate is not valid for any names, but wanted to match <hostname>` | Add the hostname to the server certificate's SANs |

### 6. Security Note
- Always use valid certificates with correct SANs in production.
- Only use `InsecureSkipVerify: true` for local development/testing.

---

For more details, see the documentation in `internal/auth/cert.go` and your server's certificate generation process.

## Available Commands

The `persysctl` supports commands to manage `workloads`, `nodes`, and `metrics`.

| Command | Description | Example |
|---------|-------------|---------|
| `persysctl schedule` | Schedule a workload on a node | `persysctl schedule --workload path/to/workload.json` |
| `persysctl workloads list` | List all workloads | `persysctl workloads list` |
| `persysctl nodes list` | List all nodes | `persysctl nodes list` |
| `persysctl metrics` | Retrieve cluster metrics | `persysctl metrics` |
| `persysctl --help` | Display help | `persysctl --help` |

## API Routes

The `persysctl` interacts with the following Prow system API endpoints:

- **POST /workloads/schedule**: Schedules a workload, returning a `ScheduleResponse` (workload ID, node ID, status).
- **GET /workloads**: Retrieves a list of workloads.
- **GET /nodes**: Retrieves a list of nodes.
- **GET /cluster/metrics**: Retrieves cluster metrics as a key-value map.

Ensure the Prow system is running and these endpoints are accessible at the configured `api_endpoint`.

## Troubleshooting
- **Configuration Errors**:
  - **Error**: `failed to read config: ...`
  - **Solution**: Verify `~/.persys/config.yaml` exists and contains `api_endpoint`. Use `--config` to specify an alternative file.
- **Connection Errors**:
  - **Error**: `failed to send request: ...`
  - **Solution**: Check the API endpoint URL and ensure the Prow system is running (`curl http://192.168.1.100:8080`).
- **Invalid Responses**:
  - **Error**: `failed to decode response: ...`
  - **Solution**: Enable verbose logging (`--verbose`) to inspect API responses. Verify the Prow system’s API version compatibility.

## Best Practices
- **Secure Configuration**: Protect `config.yaml` with `chmod 600` and avoid storing sensitive data (e.g., API keys) unless necessary.
- **Verbose Logging**: Use `--verbose` for debugging but disable it in production to reduce output.
- **Environment Variables**: Use `PERSYS_API_ENDPOINT` for temporary overrides instead of modifying `config.yaml`.
- **Version Control**: Back up `config.yaml` and track changes to avoid accidental overwrites.

## Additional Resources
- **Persys Cloud Documentation**: [Persys-cloud](https://github.com/persys-dev/persys-cloud) Refer to the Prow system’s API documentation for endpoint details.
- **Cobra Documentation**: [spf13/cobra](https://github.com/spf13/cobra) for command implementation.
- **Viper Documentation**: [spf13/viper](https://github.com/spf13/viper) for configuration management.
- **Support**: Contact the Persys development team for assistance.
