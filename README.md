# Ignis VM Worker

A high-performance, isolated code execution system using Firecracker microVMs. Ignis VM provides secure, container-like isolation for running untrusted code across multiple programming languages with built-in resource monitoring and job queuing.

## Overview

Ignis VM Worker is a backend service that manages pools of Firecracker microVMs to execute code in isolated environments. It supports multiple programming languages and provides:

- **Secure Execution**: Each code execution runs in its own isolated microVM
- **Multi-Language Support**: Python and Go (extensible to other languages)
- **Resource Monitoring**: Tracks execution time and memory usage
- **Job Queuing**: Uses NATS for reliable job distribution
- **Auto-Scaling**: Dynamic VM pool management based on demand

## Architecture

The system consists of several key components:

### Core Components

- **Worker Service** (`main.go`): Main orchestrator that manages VM pools and handles job requests
- **VM Manager** (`vm.go`, `vmm.go`): Handles Firecracker VM lifecycle (create, start, stop)
- **Pool Manager** (`pool.go`): Manages language-specific VM pools with automatic scaling
- **Job Queue** (`job_queue.go`): NATS-based job distribution and status tracking
- **Agent** (`agent-create/`): Lightweight HTTP service running inside each VM

### VM Architecture

Each microVM contains:
- Minimal Linux root filesystem with the agent service
- Language runtime (Python interpreter, Go compiler)
- HTTP API endpoint for code execution
- Resource monitoring capabilities

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   NATS Queue    │    │   Worker        │    │   Firecracker   │
│                 │    │   Service       │    │   MicroVM       │
│ Jobs →          │───▶│                 │───▶│                 │
│ Results ←       │◀───│ Pool Manager    │◀───│ Agent Service   │
└─────────────────┘    │                 │    │ (HTTP API)      │
                       │ Language Pools  │    └─────────────────┘
                       └─────────────────┘
```

## Prerequisites

### System Requirements

- Linux (kernel 4.14+ with KVM support)
- Go 1.24+
- Root/sudo access (for VM management)
- At least 4GB RAM recommended
- KVM enabled in BIOS

### Dependencies

- **Firecracker**: MicroVM hypervisor
- **NATS Server**: Message queue for job distribution
- **CNI Plugins**: For VM networking
- **Docker** (optional, for development)

## Installation

### 1. Install Firecracker

```bash
# Download and install Firecracker binary
curl -Lo firecracker https://github.com/firecracker-microvm/firecracker/releases/download/v1.0.0/firecracker-v1.0.0-x86_64.tgz
tar -xzf firecracker-v1.0.0-x86_64.tgz
sudo mv firecracker-v1.0.0-x86_64/firecracker /usr/local/bin/
sudo chmod +x /usr/local/bin/firecracker
```

### 2. Install NATS Server

```bash
# Using Docker (recommended for development)
docker run -d --name nats-server -p 4222:4222 -p 8222:8222 nats:latest -js

# Or install natively
curl -L https://github.com/nats-io/nats-server/releases/latest/download/nats-server-v2.9.0-linux-amd64.zip -o nats.zip
unzip nats.zip
sudo mv nats-server-v2.9.0-linux-amd64/nats-server /usr/local/bin/
```

### 3. Install CNI Plugins

```bash
# Install CNI plugins for networking
curl -L https://github.com/containernetworking/plugins/releases/download/v1.1.1/cni-plugins-linux-amd64-v1.1.1.tgz -o cni-plugins.tgz
sudo mkdir -p /opt/cni/bin
sudo tar -C /opt/cni/bin -xzf cni-plugins.tgz
```

### 4. Clone and Build

```bash
git clone https://github.com/AvaterClasher/ignis-vm.git
cd ignis-vm

# Build the project and VM images
make build-rootfs

# Build the worker binary
make build
```

## Configuration

### Environment Variables

Create a `.env` file in the project root or set environment variables:

```bash
# NATS connection URL
NATS_URL=nats://localhost:4222

# Path to Firecracker binary (optional, defaults to /usr/local/bin/firecracker)
FIRECRACKER_BINARY=/usr/local/bin/firecracker

# Log level (optional, defaults to info)
LOG_LEVEL=info
```

### Network Configuration

The system uses CNI networking. Ensure you have a CNI configuration file at `/etc/cni/net.d/fcnet.conflist`:

```json
{
  "cniVersion": "0.4.0",
  "name": "fcnet",
  "plugins": [
    {
      "type": "bridge",
      "bridge": "fc-br0",
      "isGateway": true,
      "ipMasq": true,
      "ipam": {
        "type": "host-local",
        "subnet": "10.0.0.0/16",
        "resolvConf": "/etc/resolv.conf"
      }
    },
    {
      "type": "firewall"
    },
    {
      "type": "tc-redirect-tap"
    }
  ]
}
```

## Usage

### Starting the Service

```bash
# Start NATS server (if not using Docker)
nats-server -js

# Start the worker
sudo ./ignis-vm
```

The worker will:
1. Discover available language rootfs images
2. Create VM pools for each language
3. Start listening for jobs on NATS
4. Automatically scale VM pools based on demand

### API Usage

Jobs are submitted via NATS. The worker listens for messages with the following JSON format:

```json
{
  "id": "unique-job-id",
  "language": "python",
  "code": "print('Hello, World!')",
  "variant": "standard"
}
```

### Supported Languages

Currently supported languages:
- **Python**: Standard Python execution
- **Go**: Go code compilation and execution

### Monitoring

- **NATS Monitoring**: Access http://localhost:8222 for NATS dashboard
- **VM Logs**: Check `/tmp/.firecracker.sock-*.log` files
- **Health Check**: VMs expose health endpoints at `:8080/health`

## Development

### Project Structure

```
.
├── main.go              # Main worker service
├── job.go              # Job handling logic
├── pool.go             # VM pool management
├── vm.go               # VM lifecycle management
├── vmm.go              # Firecracker VMM operations
├── job_queue.go        # NATS job queue
├── options.go          # VM configuration
├── agent/              # VM rootfs images
├── agent-create/       # VM image building tools
│   ├── main.go         # Agent HTTP service
│   ├── build-all.sh    # Build script
│   └── build-*.sh      # Individual build scripts
├── linux/              # Firecracker kernel
└── docker-compose.yml  # Development services
```

### Adding New Languages

1. Create a language handler in `agent-create/`
2. Add the language to the `VARIANTS` array in `build-all.sh`
3. Implement the handler logic in `main.go`
4. Build new rootfs: `make build-rootfs`

### Building Custom RootFS

```bash
cd agent-create

# Build for a specific language
VARIANT=python ./build-rootfs.sh

# Build kernel
./build-kernel.sh

# Build agent binary
OUTPUT_BIN=agent ./build-static.sh
```

## Troubleshooting

### Common Issues

**VM fails to start:**
- Ensure KVM is enabled: `lsmod | grep kvm`
- Check Firecracker binary permissions
- Verify CNI configuration

**NATS connection fails:**
- Ensure NATS server is running
- Check `NATS_URL` environment variable
- Verify network connectivity

**Rootfs not found:**
- Run `make build-rootfs` to build VM images
- Check that `agent/` directory contains rootfs files

### Logs and Debugging

```bash
# Enable debug logging
LOG_LEVEL=debug ./ignis-vm

# Check VM logs
ls /tmp/.firecracker.sock-*.log
cat /tmp/.firecracker.sock-<pid>-<vmmid>.log
```

### Resource Cleanup

```bash
# Clean up VM artifacts
make clean

# Or manually remove socket files
rm /tmp/.firecracker.sock-*
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Security Considerations

- Each VM runs in complete isolation
- Rootfs images should be minimal and regularly updated
- Network access is controlled via CNI
- Resource limits are enforced per VM
- Code execution is sandboxed within microVMs
