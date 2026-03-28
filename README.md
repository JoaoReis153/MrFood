# MrFood

## 📦 Setup
To run the services locally, Docker and Docker Compose are required.

Create your environment file:

```bash
cp services/env.tmpl services/.env
```

Update the configuration inside /services/.env as needed.

## 🔐 JWT Secret

Generate a JWT secret using:

```bash
openssl rand -base64 32
```
Add it to your `/services/.env.`

## 🚀 Running the Services

You can build and run the services using Make:

```bash
make build
make run
```
To view logs:
```bash
make logs
```
To stop services:
```bash
make stop
```

## 🧹 Cleanup

Stop and remove containers:
```bash
make down
```
Remove containers and images (project only):
```bash
make clean
```
Remove everything including volumes (⚠️ deletes data):
```bash
make clean-volumes
```
Full reset (containers, images, volumes):
```bash
make clean-all
```