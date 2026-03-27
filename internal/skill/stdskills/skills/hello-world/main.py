#!/usr/bin/env python3
import json, sys

def main():
    try:
        data = json.load(sys.stdin)
    except:
        data = {}
    name = data.get("name", "World")
    json.dump({"message": f"Hello, {name}! This is a GoEmon skill."}, sys.stdout)

if __name__ == "__main__":
    main()
