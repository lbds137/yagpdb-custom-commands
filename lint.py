#!/usr/bin/env python3
"""
Quick runner script for the YAGPDB linter
"""

import sys
import subprocess
import os

def main():
    # Get the directory of this script
    script_dir = os.path.dirname(os.path.abspath(__file__))
    linter_path = os.path.join(script_dir, "tools", "linter", "yagpdb_lint.py")
    
    # Forward all arguments to the linter
    cmd = [sys.executable, linter_path] + sys.argv[1:]
    
    # If no arguments provided, default to current directory with verbose
    if len(sys.argv) == 1:
        cmd.extend(["--dir", ".", "-v"])
    
    # Run the linter
    result = subprocess.run(cmd)
    sys.exit(result.returncode)

if __name__ == "__main__":
    main()