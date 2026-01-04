#!/usr/bin/env python3
"""
Quick runner for lint reporting
"""

import sys
import subprocess
import os

def main():
    # Get the project root (parent of scripts/ directory)
    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(script_dir)
    reporter_path = os.path.join(project_root, "tools", "linter", "lint_reporter.py")
    
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