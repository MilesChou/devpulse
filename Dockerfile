# ── Stage 1: 前端 build ──────────────────────────────────────────
FROM node:22-alpine AS frontend

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY vite.config.js tsconfig.json ./
COPY resources ./resources
RUN npm run build

# ── Stage 2: composer deps（無前端、無原始碼）──────────────────
FROM composer:2 AS vendor

WORKDIR /app

COPY composer.json composer.lock ./
RUN composer install \
        --no-dev \
        --no-autoloader \
        --no-scripts \
        --prefer-dist \
        --no-interaction \
        --no-progress

# ── Stage 3: PHP 應用（Apache + mod_php，含 bash）─────────────
FROM php:8.4-apache AS app

ENV TZ=Asia/Taipei
ENV APACHE_DOCUMENT_ROOT=/var/www/html/public

# Runtime deps：跑時要的
ARG RUNTIME_DEPS="libpq5 libzip4 ca-certificates tzdata"

# Build deps：編譯 ext 用，編完移除
ARG BUILD_DEPS="libpq-dev libzip-dev pkg-config"

RUN set -xe \
    && apt-get update \
    && apt-get install -y --no-install-recommends $RUNTIME_DEPS $BUILD_DEPS \
    && docker-php-ext-install -j "$(nproc)" \
        pdo \
        pdo_pgsql \
        opcache \
        zip \
    && apt-get purge -y --auto-remove $BUILD_DEPS \
    && rm -rf /var/lib/apt/lists/* \
    && cp /usr/share/zoneinfo/$TZ /etc/localtime \
    && echo $TZ > /etc/timezone \
    && a2enmod rewrite headers \
    && sed -ri -e 's!/var/www/html!${APACHE_DOCUMENT_ROOT}!g' /etc/apache2/sites-available/*.conf \
    && sed -ri -e 's!/var/www/!${APACHE_DOCUMENT_ROOT}!g' /etc/apache2/apache2.conf /etc/apache2/conf-available/*.conf \
    && sed -ri -e 's!Listen 80!Listen 8080!' /etc/apache2/ports.conf \
    && sed -ri -e 's!:80>!:8080>!' /etc/apache2/sites-available/*.conf \
    && php -m

WORKDIR /var/www/html

# 先帶 vendor（layer cache 只跟 composer.lock 走）
COPY --from=vendor /app/vendor ./vendor

# 帶原始碼
COPY . .

# 前端 build 產物
COPY --from=frontend /app/public/build ./public/build

# 補 autoload + 權限
COPY --from=composer:2 /usr/bin/composer /usr/bin/composer
RUN set -xe \
    && composer dump-autoload --optimize --no-dev \
    && mkdir -p storage/framework/sessions storage/framework/views storage/framework/cache \
    && chown -R www-data:www-data storage bootstrap/cache

EXPOSE 8080

CMD ["sh", "-c", "php artisan optimize && apache2-foreground"]
