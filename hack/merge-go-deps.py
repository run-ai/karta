# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
#
# Deduplicate go mod download -json output by module Path.
# Usage: python3 hack/merge-go-deps.py root-deps.json cli-deps.json > deps.json

import sys
import json

seen = {}
for fname in sys.argv[1:]:
    with open(fname) as f:
        data = f.read()
    dec = json.JSONDecoder()
    pos = 0
    while pos < len(data):
        while pos < len(data) and data[pos] in " \t\r\n":
            pos += 1
        if pos >= len(data):
            break
        obj, pos = dec.raw_decode(data, pos)
        path = obj.get("Path", "")
        if path and path not in seen:
            seen[path] = True
            print(json.dumps(obj))
