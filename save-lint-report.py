#!/usr/bin/env python3
"""
Simple lint report saver - captures linter output and saves with timestamp
"""

import subprocess
import sys
import os
from datetime import datetime
from pathlib import Path

def main():
    # Get current directory and create reports directory
    reports_dir = Path("reports")
    reports_dir.mkdir(exist_ok=True)
    
    # Get timestamp for filename
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    
    # Run linter and capture all output
    print("🔍 Running linter...")
    
    try:
        result = subprocess.run(
            [sys.executable, "tools/linter/yagpdb_lint.py", "--dir", "."],
            capture_output=True,
            text=True
        )
        
        # Combine stdout and stderr
        output = ""
        if result.stdout:
            output += result.stdout
        if result.stderr:
            if output:
                output += "\n"
            output += result.stderr
        
        # Save timestamped report
        report_file = reports_dir / f"lint_output_{timestamp}.txt"
        with open(report_file, 'w') as f:
            f.write(f"# Lint Report - {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n")
            f.write(f"# Command: python3 tools/linter/yagpdb_lint.py --dir .\n")
            f.write(f"# Exit code: {result.returncode}\n")
            f.write(f"# Git commit: {get_git_commit()}\n")
            f.write("\n")
            f.write(output)
        
        # Save as latest
        latest_file = reports_dir / "latest_lint.txt"
        with open(latest_file, 'w') as f:
            f.write(f"# Lint Report - {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n")
            f.write(f"# Command: python3 tools/linter/yagpdb_lint.py --dir .\n")
            f.write(f"# Exit code: {result.returncode}\n")
            f.write(f"# Git commit: {get_git_commit()}\n")
            f.write("\n")
            f.write(output)
        
        print(f"📄 Report saved to: {report_file}")
        print(f"📌 Latest report saved to: {latest_file}")
        
        # Also display the output
        print("\n" + "="*50)
        print(output)
        
        # Create simple summary
        lines = output.split('\n')
        error_count = sum(1 for line in lines if line.startswith('❌'))
        warning_count = sum(1 for line in lines if line.startswith('⚠️'))
        info_count = sum(1 for line in lines if line.startswith('ℹ️'))
        
        print(f"\n📊 Quick Summary:")
        print(f"   ❌ Errors: {error_count}")
        print(f"   ⚠️  Warnings: {warning_count}")
        print(f"   ℹ️  Info: {info_count}")
        print(f"   📋 Total: {error_count + warning_count + info_count}")
        
        return result.returncode
        
    except Exception as e:
        print(f"❌ Error running linter: {e}")
        return 1

def get_git_commit():
    """Get current git commit hash"""
    try:
        result = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            capture_output=True,
            text=True
        )
        return result.stdout.strip()[:8] if result.returncode == 0 else "unknown"
    except:
        return "unknown"

if __name__ == "__main__":
    sys.exit(main())