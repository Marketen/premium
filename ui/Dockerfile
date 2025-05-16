# Build stage
FROM node:20-alpine AS builder
WORKDIR /app
COPY . .
RUN npm install && npm run build

# Nginx stage
FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html

# Add proxy config
COPY nginx.conf /etc/nginx/conf.d/default.conf
