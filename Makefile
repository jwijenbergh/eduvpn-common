.PHONY: build test test-go test-wrappers clean

build:
	$(MAKE) -C exports

test: test-go test-wrappers

test-go:
	go test ./... -v

#WRAPPERS ?= $(notdir $(patsubst %/,%,$(wildcard wrappers/*/)))
WRAPPERS=python

MOCK_TARGET=epel-7-x86_64

rpm-depends:
# Setup dependencies
	echo "installing dependencies"
	dnf install -y \
		devscripts \
		golang \
		gcc \
		fedora-packager \
		fedora-review \
		python3-devel \
		python3-wheel \
		python3-setuptools \
		mock

srpm:
# Ensure tree
	rpmdev-setuptree

# Cleanup
	rm -rf dist/*

# Archive code with vendored dependencies
	git clone . dist/libeduvpn-common-2.0.0
	go mod vendor
	cp -r vendor dist/libeduvpn-common-2.0.0/vendor
	tar -zcvf ~/rpmbuild/SOURCES/libeduvpn-common.tar.gz -C dist .

# Cleanup
	rm -rf dist/*

# build SRPM and copy to dist
	rpmbuild -bs eduvpncommon.spec
	cp ~/rpmbuild/SRPMS/* dist/
	echo "Done building SRPM, go to ./dist/ to view it"

rpm: srpm
	rpmbuild -bb eduvpncommon.spec
	find ~/rpmbuild/RPMS -name '*.rpm' -exec mv {} ./dist \;
	echo "Done building RPM, go to ./dist/ to view them"

rpm-mock: srpm
	mock -r "$(MOCK_TARGET)" --resultdir ./dist rebuild ~/rpmbuild/SRPMS/libeduvpn-common*.src.rpm
	echo "Done building RPM, go to ./dist/ to view them"

# Enable parallelism if -j is specified, but first execute build
test-wrappers: build
	$(MAKE) $(foreach wrapper,$(WRAPPERS),.test-$(wrapper))

clean: .clean-libs $(foreach wrapper,$(WRAPPERS),.clean-$(wrapper))

.clean-libs:
	$(MAKE) -C exports clean

# Define test & clean for each wrapper
define wrapper_targets
.test-$(1):
	$(MAKE) -C wrappers/$(1) test
.clean-$(1):
	$(MAKE) -C wrappers/$(1) clean
endef
$(foreach wrapper,$(WRAPPERS),$(eval $(call wrapper_targets,$(wrapper))))
