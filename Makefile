# Root Makefile for the o11y multi-module monorepo. Each module builds and
# tests independently -- this just delegates. Run `make -C operator test` (or
# any module directory) to work on one module in isolation.
MODULES := operator clickhouse-migrate

.PHONY: test
test:
	@for m in $(MODULES); do \
		echo "=== $$m ==="; \
		$(MAKE) -C $$m test || exit 1; \
	done

.PHONY: build
build:
	@for m in $(MODULES); do \
		echo "=== $$m ==="; \
		$(MAKE) -C $$m build || exit 1; \
	done
