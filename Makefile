#!/usr/bin/make -f

PROCESSORS_NUM := $(shell getconf _NPROCESSORS_ONLN)

PHP_INTERPRETER ?= $(shell which php)
PHP_GLOBAL_CONFIG := -d memory_limit=-1

PHP_HAS_XDEBUG_EXTENSION := $(shell ${PHP_INTERPRETER} -r 'echo extension_loaded("xdebug") ? 1 : 0;' 2>/dev/null)
PHP_HAS_PCOV_EXTENSION := $(shell ${PHP_INTERPRETER} -r 'echo extension_loaded("pcov") ? 1 : 0;' 2>/dev/null)

ifeq ($(PHP_HAS_PCOV_EXTENSION),1)
PHP_DISABLE_COVERAGE_MODE := -d pcov.enabled=0
else ifeq ($(PHP_HAS_XDEBUG_EXTENSION),1)
PHP_DISABLE_COVERAGE_MODE := -d xdebug.mode=off
else
PHP_DISABLE_COVERAGE_MODE :=
endif

# 只跑 staged 的 php 檔案，給 fast-* target 用
CACHED_PHP_FILES := $(shell git diff --cached --name-only --diff-filter=ACM 2>/dev/null | grep -E '^(app|database|tests)/.*\.php$$')

TARGET_FILES :=

PHP := ${PHP_INTERPRETER} ${PHP_GLOBAL_CONFIG} ${PHP_DISABLE_COVERAGE_MODE}

# ---------------------------------------------------------------------

.PHONY: all
all: lint stan test

.PHONY: clean-cache
clean-cache:
	rm -rf .cache .phpstan.result.cache .phpunit.result.cache

# ---- Lint --------------------------------------------------------

.PHONY: lint
lint: lint-syntax phpcs

.PHONY: lint-syntax
lint-syntax:
	${PHP} vendor/bin/parallel-lint -j ${PROCESSORS_NUM} $(if $(TARGET_FILES),$(TARGET_FILES),app database tests)

.PHONY: phpcs
phpcs:
	@mkdir -p .cache/phpcs
	${PHP} vendor/bin/phpcs --parallel=${PROCESSORS_NUM} --cache=.cache/phpcs/.phpcs.cache ${TARGET_FILES}

.PHONY: phpcbf
phpcbf:
	@mkdir -p .cache/phpcs
	${PHP} vendor/bin/phpcbf --parallel=${PROCESSORS_NUM} --cache=.cache/phpcs/.phpcs.cache ${TARGET_FILES}

# ---- Static analysis ---------------------------------------------

.PHONY: stan
stan:
	${PHP} vendor/bin/phpstan analyse

# ---- Test --------------------------------------------------------

.PHONY: test
test:
	${PHP} artisan test --parallel --without-databases --processes=${PROCESSORS_NUM} --no-coverage --stop-on-failure --stop-on-error --passthru-php="${PHP_GLOBAL_CONFIG} ${PHP_DISABLE_COVERAGE_MODE}" ${TARGET_FILES}

# ---- Fast (staged files only) ------------------------------------

.PHONY: fast-lint
fast-lint:
ifeq ($(strip $(CACHED_PHP_FILES)),)
	@echo ">>> No staged php files under app/, database/, tests/"
else
	${PHP} vendor/bin/parallel-lint -j ${PROCESSORS_NUM} ${CACHED_PHP_FILES}
endif

.PHONY: fast-phpcs
fast-phpcs:
ifeq ($(strip $(CACHED_PHP_FILES)),)
	@echo ">>> No staged php files under app/, database/, tests/"
else
	@mkdir -p .cache/phpcs
	${PHP} vendor/bin/phpcs --parallel=${PROCESSORS_NUM} --cache=.cache/phpcs/.phpcs.cache ${CACHED_PHP_FILES}
endif

.PHONY: fast-phpcbf
fast-phpcbf:
ifeq ($(strip $(CACHED_PHP_FILES)),)
	@echo ">>> No staged php files under app/, database/, tests/"
else
	@mkdir -p .cache/phpcs
	${PHP} vendor/bin/phpcbf --parallel=${PROCESSORS_NUM} --cache=.cache/phpcs/.phpcs.cache ${CACHED_PHP_FILES}
endif

.PHONY: fast-stan
fast-stan:
ifeq ($(strip $(CACHED_PHP_FILES)),)
	@echo ">>> No staged php files under app/, database/, tests/"
else
	${PHP} vendor/bin/phpstan analyse ${CACHED_PHP_FILES}
endif
