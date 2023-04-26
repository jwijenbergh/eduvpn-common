# Example: commands to build for Python
This section gives an example on how to build and install the library from scratch (assuming you have all the dependencies). It builds the Go library and then builds and installs the Python wrapper.

1. Clone the library
```bash
git clone https://github.com/eduvpn/eduvpn-common
```

2. Go to the library directory
```bash
cd eduvpn-common
```

3. Build the go library
```bash
make
```

4. Build the python wrapper
```bash
make -C wrappers/python
```

5. Install the wheel using pip
```bash
pip install wrappers/python/dist/eduvpncommon-2.0.0-py3-none-linux_x86_64.whl
```
Note that the name of your wheel changes on the platform and version.
