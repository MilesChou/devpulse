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

# ── Stage 3: PHP 應用 ────────────────────────────────────────────
FROM php:8.4-fpm-alpine AS app

ENV TZ=Asia/Taipei

# Runtime deps：跑時要的
ENV RUNTIME_DEPS \
    nginx \
    curl \
    libpq \
    tzdata

# Build deps：編譯 ext 用，編完即拆
ENV BUILD_DEPS \
    curl-dev \
    postgresql-dev

RUN set -xe \
    && apk add --no-cache $RUNTIME_DEPS \
    && apk add --no-cache --virtual .build-deps $PHPIZE_DEPS $BUILD_DEPS \
    && docker-php-ext-install -j "$(getconf _NPROCESSORS_ONLN)" \
        pdo \
        pdo_pgsql \
        opcache \
        curl \
    && apk del --no-network .build-deps \
    && cp /usr/share/zoneinfo/$TZ /etc/localtime \
    && echo $TZ > /etc/timezone \
    && php -m

WORKDIR /var/www/html

# 先帶 vendor（內容只跟 composer.lock 走，layer cache 命中率高）
COPY --from=vendor /app/vendor ./vendor

# 再帶原始碼，讓 vendor layer 在原始碼變動時不必重做
COPY . .

# 拷貝前端 build 產物
COPY --from=frontend /app/public/build ./public/build

# 補 autoload（含應用 class）+ 權限 + nginx 設定
COPY --from=composer:2 /usr/bin/composer /usr/bin/composer
RUN set -xe \
    && composer dump-autoload --optimize --no-dev \
    && mkdir -p storage/framework/sessions storage/framework/views storage/framework/cache \
    && chown -R www-data:www-data storage bootstrap/cache

COPY .docker/nginx.conf /etc/nginx/nginx.conf

EXPOSE 8080

CMD ["sh", "-c", "php artisan optimize && php artisan migrate --force && php-fpm -D && nginx -g 'daemon off;'"]
