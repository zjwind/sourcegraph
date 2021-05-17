#!/bin/bash

LSIF_CLANG="$HOME/sourcegraph/llvm-project/clang-tools-extra/lsif-clang/build/lsif-clang"
# LSIF_CLANG="lsif-clang"

go run . --indexer "$LSIF_CLANG compile_commands" --dir "$HOME/sourcegraph/lsif-clang/functionaltest/" --debug
