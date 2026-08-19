#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2026 NVIDIA Corporation
"""Extract the rule groups of every PrometheusRule in a rendered Helm
manifest stream (stdin) as a plain Prometheus rules file (stdout)."""

import sys

import yaml


def main() -> None:
    groups = []
    for doc in yaml.safe_load_all(sys.stdin):
        if doc and doc.get("kind") == "PrometheusRule":
            groups.extend(doc["spec"]["groups"])
    if not groups:
        sys.exit("no PrometheusRule found in the rendered manifests")
    yaml.safe_dump({"groups": groups}, sys.stdout, sort_keys=False)


if __name__ == "__main__":
    main()
