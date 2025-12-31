# ---------- 基础配置 ----------
PROTOC        := protoc
PROTO_DIR     := proto
OUT_DIR       := gossip_rpc_chunk
PROTO_FILES   := $(PROTO_DIR)/gossip_rpc_chunk.proto

# ---------- 默认目标 ----------
.PHONY: proto
proto:
	$(PROTOC) \
		--go_out=$(OUT_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(OUT_DIR) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)
# 用gitbash自带的mingw32的makefile