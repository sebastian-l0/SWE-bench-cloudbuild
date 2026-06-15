SHELL := /usr/bin/env bash

.PHONY: tdd pre-commit pre-push check check-openspec check-secrets check-shell install-hooks lefthook-install

tdd:
	./scripts/check.sh tdd

pre-commit:
	./scripts/check.sh pre-commit

pre-push:
	./scripts/check.sh pre-push

check: pre-commit

check-openspec:
	./scripts/check-openspec.sh

check-secrets:
	./scripts/check-secrets.sh

check-shell:
	./scripts/check-shell.sh

install-hooks: lefthook-install

lefthook-install:
	lefthook install --reset-hooks-path
	chmod +x scripts/*.sh test/*.sh
	@echo "Git hooks installed with lefthook"
