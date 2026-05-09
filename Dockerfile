# ── Stage 1: 前端 build ──────────────────────────────────────────
FROM node:22-alpine AS frontend

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY vite.config.js tsconfig.json ./
COPY resources ./resources
RUN npm run build

# ── Stage 2: PHP 應用 ────────────────────────────────────────────
FROM php:8.3-fpm-alpine AS app

RUN apk add --no-cache \
        nginx \
        curl \
        postgresql-dev \
    && docker-php-ext-install pdo pdo_pgsql opcache curl

COPY --from=composer:2 /usr/bin/composer /usr/bin/composer

WORKDIR /var/www/html

COPY composer.json composer.lock ./
RUN composer install --no-dev --no-autoloader --no-scripts --prefer-dist

COPY . .

RUN composer dump-autoload --optimize

COPY --from=frontend /app/public/build ./public/build

RUN mkdir -p storage/framework/{sessions,views,cache} \
    && chown -R www-data:www-data storage bootstrap/cache

COPY .docker/nginx.conf /etc/nginx/nginx.conf

EXPOSE 8080

CMD ["sh", "-c", "php artisan optimize && php artisan migrate --force && php-fpm -D && nginx -g 'daemon off;'"]
