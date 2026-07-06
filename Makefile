.DEFAULT_GOAL := help

help:
	@mise tasks

%:
	@mise run $@
