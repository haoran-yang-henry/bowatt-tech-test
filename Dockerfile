# ---- build stage ----
FROM node:22-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
# Empty base URL -> the frontend uses relative /api paths, which nginx proxies
# to the backend, so the browser talks to a single origin (no CORS needed).
ENV VITE_API_BASE_URL=""
RUN npm run build

# ---- run stage ----
FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
