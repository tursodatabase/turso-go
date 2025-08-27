#!/usr/bin/env python3
#
# Copyright 2025 the Turso authors. All rights reserved. MIT license.
#
# A script to merge a pull requests with a nice merge commit using GitHub CLI.
#
# Requirements:
# - GitHub CLI (`gh`) must be installed and authenticated
import json
import os
import re
import shlex
import subprocess
import sys
import tempfile
import textwrap


def run_command(command, capture_output=True):
    if capture_output:
        process = subprocess.Popen(
            command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, shell=True
        )
        output, error = process.communicate()
        return (
            output.decode("utf-8").strip(),
            error.decode("utf-8").strip(),
            process.returncode,
        )
    else:
        return "", "", subprocess.call(command, shell=True)


def load_user_mapping(file_path=".github.json"):
    if os.path.exists(file_path):
        with open(file_path, "r") as f:
            return json.load(f)
    return {}


user_mapping = load_user_mapping()


def get_user_email(username):
    if username in user_mapping:
        return f"{user_mapping[username]['name']} <{user_mapping[username]['email']}>"

    # Try to get user info from gh CLI
    output, _, returncode = run_command(f"gh api users/{username}")
    if returncode == 0:
        user_data = json.loads(output)
        name = user_data.get("name", username)
        email = user_data.get("email")
        if email:
            return f"{name} <{email}>"
        return f"{name} (@{username})"

    # Fallback to noreply address
    return f"{username} <{username}@users.noreply.github.com>"


def get_pr_info(pr_number):
    output, error, returncode = run_command(
        f"gh pr view {pr_number} --json number,title,author,headRefName,body,reviews"
    )
    if returncode != 0:
        print(f"Error fetching PR #{pr_number}: {error}")
        sys.exit(1)

    pr_data = json.loads(output)

    reviewed_by = []
    for review in pr_data.get("reviews", []):
        if review["state"] == "APPROVED":
            reviewed_by.append(get_user_email(review["author"]["login"]))

    # Remove duplicates while preserving order
    reviewed_by = list(dict.fromkeys(reviewed_by))

    return {
        "number": pr_data["number"],
        "title": pr_data["title"],
        "author": pr_data["author"]["login"],
        "author_name": pr_data["author"].get("name", pr_data["author"]["login"]),
        "head": pr_data["headRefName"],
        "body": (pr_data.get("body") or "").strip(),
        "reviewed_by": reviewed_by,
    }


def wrap_text(text, width=72):
    lines = text.split("\n")
    wrapped_lines = []
    in_code_block = False
    for line in lines:
        if line.strip().startswith("```"):
            in_code_block = not in_code_block
            wrapped_lines.append(line)
        elif in_code_block:
            wrapped_lines.append(line)
        else:
            wrapped_lines.extend(textwrap.wrap(line, width=width))
    return "\n".join(wrapped_lines)


def merge_remote(pr_number: int, commit_message: str, commit_title: str):
    output, error, returncode = run_command(f"gh pr checks {pr_number} --json state")
    if returncode == 0:
        checks_data = json.loads(output)
        if checks_data and any(
            check.get("state") == "FAILURE" for check in checks_data
        ):
            print("Warning: Some checks are failing")
            if (
                input("Do you want to proceed with the merge? (y/N): ").strip().lower()
                != "y"
            ):
                exit(0)

    # Create a temporary file for the commit message
    with tempfile.NamedTemporaryFile(
        mode="w", suffix=".txt", delete=False
    ) as temp_file:
        temp_file.write(commit_message)
        temp_file_path = temp_file.name

    try:
        print(f"\nMerging PR #{pr_number} with custom commit message...")

        # Use gh pr merge with the commit message file
        safe_title = shlex.quote(commit_title)
        cmd = f'gh pr merge {pr_number} --merge --subject {safe_title} --body-file "{temp_file_path}"'
        output, error, returncode = run_command(cmd, capture_output=False)

        if returncode == 0:
            print(f"\nPull request #{pr_number} merged successfully!")
            print(f"\nMerge commit message:\n{commit_message}")
        else:
            print(f"Error merging PR: {error}")
            sys.exit(1)
    finally:
        # Clean up the temporary file
        os.unlink(temp_file_path)


def merge_local(pr_number: int, commit_message: str):
    current_branch, _, _ = run_command("git branch --show-current")

    print(f"Fetching PR #{pr_number}...")
    cmd = f"gh pr checkout {pr_number}"
    _, error, returncode = run_command(cmd)
    if returncode != 0:
        print(f"Error checking out PR: {error}")
        sys.exit(1)

    pr_branch, _, _ = run_command("git branch --show-current")

    cmd = "git checkout main"
    _, error, returncode = run_command(cmd)
    if returncode != 0:
        print(f"Error checking out main branch: {error}")
        sys.exit(1)

    with tempfile.NamedTemporaryFile(mode="w", delete=False) as temp_file:
        temp_file.write(commit_message)
        temp_file_path = temp_file.name

    try:
        # Merge the PR branch with the custom message
        # Using -F with the full message (title + body)
        cmd = f"git merge --no-ff {pr_branch} -F {temp_file_path}"
        _, error, returncode = run_command(cmd)
        if returncode != 0:
            print(f"Error merging PR: {error}")
            # Try to go back to original branch
            run_command(f"git checkout {current_branch}")
            sys.exit(1)

        print("\nPull request merged successfully locally!")
        print(f"\nMerge commit message:\n{commit_message}")

    finally:
        # Clean up the temporary file
        os.unlink(temp_file_path)


def merge_pr(pr_number, use_api=True):
    """Merge a pull request with a formatted commit message"""
    check_gh_auth()

    print(f"Fetching PR #{pr_number}...")
    pr_info = get_pr_info(pr_number)
    print(f"PR found: '{pr_info['title']}' by {pr_info['author']}")

    # Format commit message
    commit_title = f"Merge '{pr_info['title']}' from {pr_info['author_name']}"
    commit_body = wrap_text(pr_info["body"])

    commit_message_parts = [commit_title]
    if commit_body:
        commit_message_parts.append("")  # Empty line between title and body
        commit_message_parts.append(commit_body)
    if pr_info["reviewed_by"]:
        commit_message_parts.append("")  # Empty line before reviewed-by
        for approver in pr_info["reviewed_by"]:
            commit_message_parts.append(f"Reviewed-by: {approver}")
    commit_message_parts.append("")  # Empty line before Closes
    commit_message_parts.append(f"Closes #{pr_info['number']}")
    commit_message = "\n".join(commit_message_parts)

    if use_api:
        # For remote merge, we need to separate title from body
        commit_body_for_api = "\n".join(commit_message_parts[2:])
        merge_remote(pr_number, commit_body_for_api, commit_title)
    else:
        merge_local(pr_number, commit_message)


def check_gh_auth():
    """Check if gh CLI is authenticated"""
    _, _, returncode = run_command("gh auth status")
    if returncode != 0:
        print("Error: GitHub CLI is not authenticated. Run 'gh auth login' first.")
        sys.exit(1)


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(
        description="Merge a pull request with a nice merge commit using GitHub CLI"
    )
    parser.add_argument("pr_number", type=str, help="Pull request number to merge")
    parser.add_argument(
        "--local",
        action="store_true",
        help="Use local git commands instead of GitHub API",
    )
    args = parser.parse_args()
    if not re.match(r"^\d+$", args.pr_number):
        print("Error: PR number must be a positive integer")
        sys.exit(1)
    use_api = not args.local
    merge_pr(args.pr_number, use_api)
