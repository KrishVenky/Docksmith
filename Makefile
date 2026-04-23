CC      = gcc
CFLAGS  = -std=gnu11 -O2 -Wall -Wextra \
           -Ivendor/cjson \
           -Ic_src \
           $(shell pkg-config --cflags openssl 2>/dev/null || echo "-I/usr/local/opt/openssl/include -I/opt/homebrew/opt/openssl/include")
LDFLAGS = $(shell pkg-config --libs openssl 2>/dev/null || echo "-L/usr/local/opt/openssl/lib -L/opt/homebrew/opt/openssl/lib") -lssl -lcrypto

TARGET  = docksmith_c
GO_TARGET = docksmith
BASE_IMAGE = alpine:3.19
BASE_TAR = alpine-minirootfs-3.19.1-x86_64.tar
DEMO_IMAGE = demo:v1

SRCS = \
	vendor/cjson/cJSON.c \
	c_src/util/hash.c \
	c_src/util/tar.c \
	c_src/store/store.c \
	c_src/store/layer.c \
	c_src/store/image.c \
	c_src/build/parser.c \
	c_src/build/cache.c \
	c_src/build/engine.c \
	c_src/container/run.c \
	c_src/cmd/commands.c \
	c_src/main.c

OBJS = $(SRCS:.c=.o)

.PHONY: all clean clean-all go smoke smoke-c

all: $(TARGET) go

$(TARGET): $(OBJS)
	$(CC) $(OBJS) $(LDFLAGS) -o $@
	@echo "Built $(TARGET)"

%.o: %.c
	$(CC) $(CFLAGS) -c $< -o $@

go:
	go build -o $(GO_TARGET) .

smoke: all
	@if [ ! -f "$(BASE_TAR)" ]; then echo "Missing $(BASE_TAR)"; exit 1; fi
	sudo ./$(GO_TARGET) import $(BASE_IMAGE) $(BASE_TAR)
	sudo ./$(GO_TARGET) build --no-cache -t $(DEMO_IMAGE) -f Docksmithfile .
	sudo ./$(GO_TARGET) run -e GREETING=hello $(DEMO_IMAGE)
	sudo ./$(GO_TARGET) images
	sudo ./$(GO_TARGET) cache

smoke-c: $(TARGET)
	@if [ ! -f "$(BASE_TAR)" ]; then echo "Missing $(BASE_TAR)"; exit 1; fi
	sudo ./$(TARGET) import $(BASE_IMAGE) $(BASE_TAR)
	sudo ./$(TARGET) build --no-cache -t $(DEMO_IMAGE) -f Docksmithfile .
	sudo ./$(TARGET) run -e GREETING=hello $(DEMO_IMAGE)
	sudo ./$(TARGET) images
	sudo ./$(TARGET) cache

clean:
	rm -f $(OBJS) $(TARGET)

clean-all: clean
	rm -f $(GO_TARGET)
