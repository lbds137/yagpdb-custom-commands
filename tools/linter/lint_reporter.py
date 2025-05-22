#!/usr/bin/env python3
"""
Lint Reporter - Stores and tracks linting results over time
"""

import json
import os
import sys
import subprocess
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Any


class LintReporter:
    def __init__(self, reports_dir: str = "reports"):
        self.reports_dir = Path(reports_dir)
        self.reports_dir.mkdir(exist_ok=True)
    
    def run_linter_and_capture(self, linter_path: str, target_dir: str = ".") -> Dict[str, Any]:
        """Run the linter and capture structured output"""
        try:
            # Run linter and capture output
            result = subprocess.run(
                [sys.executable, linter_path, "--dir", target_dir],
                capture_output=True,
                text=True
            )
            
            # Parse the output to extract structured information
            output_lines = result.stdout.strip().split('\n') if result.stdout else []
            error_lines = result.stderr.strip().split('\n') if result.stderr else []
            all_lines = output_lines + error_lines
            
            issues = []
            summary = {"errors": 0, "warnings": 0, "info": 0}
            
            for line in all_lines:
                if line.startswith(('❌', '⚠️', 'ℹ️')):
                    # Parse issue line: "❌ file:line:col [rule] message"
                    parts = line.split(' ', 1)
                    if len(parts) >= 2:
                        severity_icon = parts[0]
                        rest = parts[1]
                        
                        # Extract file:line:col
                        location_end = rest.find(' [')
                        if location_end > 0:
                            location = rest[:location_end]
                            rule_and_message = rest[location_end + 2:]  # Skip ' ['
                            
                            # Extract rule and message
                            rule_end = rule_and_message.find('] ')
                            if rule_end > 0:
                                rule = rule_and_message[:rule_end]
                                message = rule_and_message[rule_end + 2:]
                                
                                # Determine severity
                                severity = "error" if severity_icon == "❌" else "warning" if severity_icon == "⚠️" else "info"
                                
                                # Parse location
                                location_parts = location.split(':')
                                if len(location_parts) >= 3:
                                    file_path = ':'.join(location_parts[:-2])  # Handle paths with colons
                                    line_num = location_parts[-2]
                                    col_num = location_parts[-1]
                                    
                                    issues.append({
                                        "file": file_path,
                                        "line": int(line_num),
                                        "column": int(col_num),
                                        "rule": rule,
                                        "message": message,
                                        "severity": severity
                                    })
                                    
                                    summary[severity] += 1
                
                elif line.startswith("📊 Summary:"):
                    # Parse summary line to double-check counts
                    pass
            
            return {
                "timestamp": datetime.now().isoformat(),
                "git_commit": self.get_git_commit(),
                "total_files": self.count_gohtml_files(target_dir),
                "summary": summary,
                "issues": issues,
                "exit_code": result.returncode
            }
            
        except Exception as e:
            return {
                "timestamp": datetime.now().isoformat(),
                "error": str(e),
                "exit_code": -1
            }
    
    def get_git_commit(self) -> str:
        """Get current git commit hash"""
        try:
            result = subprocess.run(
                ["git", "rev-parse", "HEAD"],
                capture_output=True,
                text=True,
                cwd="."
            )
            return result.stdout.strip() if result.returncode == 0 else "unknown"
        except:
            return "unknown"
    
    def count_gohtml_files(self, target_dir: str) -> int:
        """Count .gohtml files in target directory"""
        try:
            path = Path(target_dir)
            return len(list(path.rglob("*.gohtml")))
        except:
            return 0
    
    def save_report(self, report: Dict[str, Any], filename: str = None) -> str:
        """Save report to JSON file"""
        if filename is None:
            timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
            filename = f"lint_report_{timestamp}.json"
        
        report_path = self.reports_dir / filename
        
        with open(report_path, 'w') as f:
            json.dump(report, f, indent=2)
        
        return str(report_path)
    
    def save_latest_report(self, report: Dict[str, Any]) -> str:
        """Save report as the latest report"""
        return self.save_report(report, "latest.json")
    
    def load_report(self, filename: str) -> Dict[str, Any]:
        """Load a report from file"""
        report_path = self.reports_dir / filename
        
        with open(report_path, 'r') as f:
            return json.load(f)
    
    def generate_summary(self, report: Dict[str, Any]) -> str:
        """Generate human-readable summary"""
        if "error" in report:
            return f"❌ Lint failed: {report['error']}"
        
        summary = report["summary"]
        timestamp = datetime.fromisoformat(report["timestamp"]).strftime("%Y-%m-%d %H:%M:%S")
        
        lines = [
            f"📊 Lint Report - {timestamp}",
            f"📁 Files scanned: {report['total_files']} .gohtml files",
            f"🔧 Git commit: {report['git_commit'][:8]}",
            "",
            f"📈 Results:",
            f"  ❌ Errors: {summary['errors']}",
            f"  ⚠️  Warnings: {summary['warnings']}",
            f"  ℹ️  Info: {summary['info']}",
            f"  📋 Total issues: {sum(summary.values())}"
        ]
        
        if report.get("issues"):
            lines.extend([
                "",
                "🔝 Top issues by rule:"
            ])
            
            # Count issues by rule
            rule_counts = {}
            for issue in report["issues"]:
                rule = issue["rule"]
                rule_counts[rule] = rule_counts.get(rule, 0) + 1
            
            # Sort by count
            sorted_rules = sorted(rule_counts.items(), key=lambda x: x[1], reverse=True)
            
            for rule, count in sorted_rules[:5]:  # Top 5
                lines.append(f"  • {rule}: {count}")
        
        return "\n".join(lines)
    
    def compare_reports(self, old_filename: str, new_filename: str) -> str:
        """Compare two reports and show changes"""
        try:
            old_report = self.load_report(old_filename)
            new_report = self.load_report(new_filename)
            
            old_summary = old_report["summary"]
            new_summary = new_report["summary"]
            
            lines = [
                "📊 Lint Comparison",
                f"🔄 {old_filename} → {new_filename}",
                ""
            ]
            
            for category in ["errors", "warnings", "info"]:
                old_count = old_summary.get(category, 0)
                new_count = new_summary.get(category, 0)
                diff = new_count - old_count
                
                if diff > 0:
                    icon = "📈"
                    sign = "+"
                elif diff < 0:
                    icon = "📉"
                    sign = ""
                else:
                    icon = "➡️"
                    sign = ""
                
                lines.append(f"{icon} {category.title()}: {old_count} → {new_count} ({sign}{diff})")
            
            return "\n".join(lines)
            
        except Exception as e:
            return f"❌ Error comparing reports: {e}"
    
    def generate_markdown_report(self, report: Dict[str, Any]) -> str:
        """Generate markdown report for documentation"""
        if "error" in report:
            return f"# Lint Report\n\n❌ **Error**: {report['error']}\n"
        
        summary = report["summary"]
        timestamp = datetime.fromisoformat(report["timestamp"]).strftime("%Y-%m-%d %H:%M:%S")
        
        md_lines = [
            "# Code Quality Report",
            "",
            f"**Generated**: {timestamp}  ",
            f"**Files Scanned**: {report['total_files']} `.gohtml` files  ",
            f"**Git Commit**: `{report['git_commit'][:8]}`  ",
            "",
            "## Summary",
            "",
            f"| Severity | Count |",
            f"|----------|-------|",
            f"| ❌ Errors | {summary['errors']} |",
            f"| ⚠️ Warnings | {summary['warnings']} |",
            f"| ℹ️ Info | {summary['info']} |",
            f"| **Total** | **{sum(summary.values())}** |",
            ""
        ]
        
        if report.get("issues"):
            # Group issues by severity and rule
            issues_by_severity = {"error": [], "warning": [], "info": []}
            for issue in report["issues"]:
                issues_by_severity[issue["severity"]].append(issue)
            
            for severity in ["error", "warning", "info"]:
                issues = issues_by_severity[severity]
                if not issues:
                    continue
                
                icon = "❌" if severity == "error" else "⚠️" if severity == "warning" else "ℹ️"
                md_lines.extend([
                    f"## {icon} {severity.title()}s",
                    ""
                ])
                
                # Group by rule
                by_rule = {}
                for issue in issues:
                    rule = issue["rule"]
                    if rule not in by_rule:
                        by_rule[rule] = []
                    by_rule[rule].append(issue)
                
                for rule, rule_issues in by_rule.items():
                    md_lines.extend([
                        f"### `{rule}` ({len(rule_issues)} issues)",
                        ""
                    ])
                    
                    for issue in rule_issues[:10]:  # Limit to 10 per rule
                        md_lines.append(f"- **{issue['file']}:{issue['line']}** - {issue['message']}")
                    
                    if len(rule_issues) > 10:
                        md_lines.append(f"- *... and {len(rule_issues) - 10} more*")
                    
                    md_lines.append("")
        
        return "\n".join(md_lines)


def main():
    import argparse
    
    parser = argparse.ArgumentParser(description="YAGPDB Lint Reporter")
    parser.add_argument("--linter", default="yagpdb_lint.py", help="Path to linter script")
    parser.add_argument("--dir", default=".", help="Directory to lint")
    parser.add_argument("--output", help="Output filename (default: timestamped)")
    parser.add_argument("--latest", action="store_true", help="Also save as latest.json")
    parser.add_argument("--markdown", action="store_true", help="Generate markdown report")
    parser.add_argument("--compare", help="Compare with previous report")
    parser.add_argument("--summary", action="store_true", help="Show summary of latest report")
    
    args = parser.parse_args()
    
    reporter = LintReporter()
    
    if args.summary:
        try:
            latest = reporter.load_report("latest.json")
            print(reporter.generate_summary(latest))
        except FileNotFoundError:
            print("❌ No latest report found. Run linter first.")
        return
    
    if args.compare:
        try:
            comparison = reporter.compare_reports(args.compare, "latest.json")
            print(comparison)
        except Exception as e:
            print(f"❌ Error comparing reports: {e}")
        return
    
    # Get linter path
    script_dir = Path(__file__).parent
    linter_path = script_dir / args.linter
    
    # Run linter and generate report
    print(f"🔍 Running linter on {args.dir}...")
    report = reporter.run_linter_and_capture(str(linter_path), args.dir)
    
    # Save report
    report_path = reporter.save_report(report, args.output)
    print(f"💾 Report saved to: {report_path}")
    
    if args.latest:
        latest_path = reporter.save_latest_report(report)
        print(f"📌 Latest report saved to: {latest_path}")
    
    # Generate markdown if requested
    if args.markdown:
        md_content = reporter.generate_markdown_report(report)
        md_path = report_path.replace('.json', '.md')
        with open(md_path, 'w') as f:
            f.write(md_content)
        print(f"📝 Markdown report saved to: {md_path}")
    
    # Show summary
    print("\n" + reporter.generate_summary(report))


if __name__ == "__main__":
    main()