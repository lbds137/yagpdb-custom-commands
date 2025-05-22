#!/usr/bin/env python3
"""
Quick runner for lint reporting
"""

import sys
import subprocess
import os

def main():
    # Get the directory of this script
    script_dir = os.path.dirname(os.path.abspath(__file__))
    reporter_path = os.path.join(script_dir, "tools", "linter", "lint_reporter.py")
    
    # Forward all arguments to the reporter
    cmd = [sys.executable, reporter_path] + sys.argv[1:]
    
    # If no arguments provided, run with default options
    if len(sys.argv) == 1:
        cmd.extend(["--latest", "--markdown"])
    
    # Run the reporter
    result = subprocess.run(cmd)
    sys.exit(result.returncode)

if __name__ == "__main__":
    main()