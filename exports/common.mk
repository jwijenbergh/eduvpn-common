# Prevent executing `go env ...` multiple times for the same property
# export is needed for this and also to pass the values on to the Go compiler
ifndef GOOS
export GOOS := $(shell go env GOHOSTOS)
endif
ifndef GOARCH
export GOARCH := $(shell go env GOHOSTARCH)
endif

ifeq (windows,$(GOOS))
LIB_PREFIX ?=
LIB_SUFFIX ?= .dll
else ifeq (darwin,$(GOOS))
LIB_PREFIX ?= lib
LIB_SUFFIX ?= .dylib
else
LIB_PREFIX ?= lib
LIB_SUFFIX ?= .so
endif

# Current version
VERSION = 2.0.0

# Library name without prefixes/suffixes
LIB_NAME ?= eduvpn_common
LIB_NAME_VERSION ?= $(LIB_NAME)-$(VERSION)
# Library file name
LIB_FILE ?= $(LIB_PREFIX)$(LIB_NAME_VERSION)$(LIB_SUFFIX)

# Get relative exports/ directory when included from a wrapper, without trailing slash
override EXPORTS_PATH = $(patsubst %/,%,$(dir $(lastword $(MAKEFILE_LIST))))

EXPORTS_LIB_PATH ?= $(EXPORTS_PATH)/lib
EXPORTS_LIB_SUBFOLDER_PATH ?= $(EXPORTS_LIB_PATH)/$(GOOS)/$(GOARCH)

# Add library to dynamic linker path for running tests
ifeq (Windows_NT,$(OS))
export PATH := $(abspath $(EXPORTS_LIB_SUBFOLDER_PATH)):$(PATH)
else
export LD_LIBRARY_PATH := $(abspath $(EXPORTS_LIB_SUBFOLDER_PATH)):$(LD_LIBRARY_PATH)
export DYLD_FALLBACK_LIBRARY_PATH := $(abspath $(EXPORTS_LIB_SUBFOLDER_PATH)):$(DYLD_FALLBACK_LIBRARY_PATH)
endif

.try-build-lib:
ifneq ($(wildcard $(EXPORTS_PATH)/Makefile),)
	$(MAKE) -C $(EXPORTS_PATH)
else
$(info Skipping building library as exports/Makefile was not found)
endif
