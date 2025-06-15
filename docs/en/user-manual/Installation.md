# Installation Guide

::: danger IMPORTANT
Read [App Compatibility](./user-manual-overview.md#compatibility) First!
:::
## Install Docker
### Windows
1. Download [Docker Desktop](https://www.docker.com/products/docker-desktop/) for Windows
2. Install and follow the setup wizard
3. Start Docker Desktop
4. Open PowerShell and verify installation:
```powershell
docker --version
docker-compose --version
```

### Mac
1. Download [Docker Desktop](https://www.docker.com/products/docker-desktop/) for Mac
2. Install the .dmg package
3. Start Docker Desktop from Applications
4. Open Terminal and verify installation:
```bash
docker --version
docker-compose --version
```

### Linux
1. Update package index:
```bash
sudo apt update
```
2. Install dependencies:
```bash
sudo apt install apt-transport-https ca-certificates curl software-properties-common
```
3. Add Docker's GPG key:
```bash
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
```
4. Add Docker repository:
```bash
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list
```
5. Install Docker:
```bash
sudo apt update
sudo apt install docker-ce docker-ce-cli containerd.io docker-compose-plugin
```
6. Verify installation:
```bash
docker --version
docker compose version
```

## Docker-Compose

### lumilio-app

### lumilio-web

### lumilio-db

### lumilio-ml

## CPU Acceleration
### X86 (AVX)

### ARM (SIMD)

## GPU Acceleration
### NVIDIA (CUDA)

### AMD (ROCm)

### Intel (DLBoost)
waitlist

## NPU Acceleration
### RockChip (RNNX)

### Apple (ANE)

### Qualcomm (Hexagon DSP)
waitlist

### Google (TPU)
waitlist



