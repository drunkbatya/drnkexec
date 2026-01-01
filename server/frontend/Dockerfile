FROM node:25-alpine3.22 AS builder
WORKDIR /app/drnkexec
COPY src/package*.json ./
RUN npm install
COPY src .
RUN npm run build

FROM nginx:1.29.3-alpine3.22
COPY --from=builder /app/drnkexec/dist /usr/share/nginx/html
