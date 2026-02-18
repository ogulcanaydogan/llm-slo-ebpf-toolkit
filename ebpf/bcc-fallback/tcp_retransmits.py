#!/usr/bin/env python3
"""Placeholder BCC fallback script for TCP retransmit signal collection."""

import json
import time


def main() -> None:
    sample = {
        "signal": "tcp_retransmits_total",
        "mode": "bcc_fallback",
        "ts_unix_nano": int(time.time() * 1_000_000_000),
        "value": 0,
    }
    print(json.dumps(sample))


if __name__ == "__main__":
    main()
