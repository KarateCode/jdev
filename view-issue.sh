#!/bin/bash
# View a Jira issue with AI Analysis support for EST tickets
# Usage: view-issue.sh <ISSUE-KEY> [--comments N]

ISSUE_KEY="$1"
shift
EXTRA_ARGS="$@"

# Function to extract plain text from Atlassian Document Format (ADF)
extract_adf_text() {
    jq -r '
    def extract_text:
        if type == "object" then
            if .type == "text" then
                .text // ""
            elif .type == "hardBreak" then
                "\n"
            elif .type == "rule" then
                "\n---\n"
            elif .type == "heading" then
                "\n" + (if .attrs.level == 1 then "# "
                       elif .attrs.level == 2 then "## "
                       elif .attrs.level == 3 then "### "
                       elif .attrs.level == 4 then "#### "
                       else "##### " end) + (.content // [] | map(extract_text) | join("")) + "\n"
            elif .type == "paragraph" then
                (.content // [] | map(extract_text) | join("")) + "\n"
            elif .type == "bulletList" then
                (.content // [] | map(extract_text) | join(""))
            elif .type == "orderedList" then
                (.content // [] | map(extract_text) | join(""))
            elif .type == "listItem" then
                "  • " + (.content // [] | map(extract_text) | join("")) + "\n"
            elif .type == "codeBlock" then
                "\n```\n" + (.content // [] | map(extract_text) | join("")) + "```\n"
            elif .type == "blockquote" then
                "> " + (.content // [] | map(extract_text) | join(""))
            else
                (.content // [] | map(extract_text) | join(""))
            end
        elif type == "array" then
            map(extract_text) | join("")
        else
            ""
        end;
    extract_text
    ' 2>/dev/null
}

# Show the standard jira view first (without comments initially)
jira issue view "$ISSUE_KEY" --plain

# Check if this is an EST ticket
if [[ "$ISSUE_KEY" == EST-* ]]; then
    # Fetch the AI Analysis field
    AI_ANALYSIS=$(jira issue view "$ISSUE_KEY" --raw 2>/dev/null | jq -r '.fields.customfield_11723 // empty')
    
    if [[ -n "$AI_ANALYSIS" && "$AI_ANALYSIS" != "null" ]]; then
        echo ""
        echo -e "\033[1;36m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m"
        echo -e "\033[1;33m📊 AI Analysis\033[0m"
        echo -e "\033[1;36m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m"
        echo ""
        echo "$AI_ANALYSIS" | extract_adf_text
        echo ""
    fi
fi

# Now show comments
echo -e "\033[1;36m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m"
echo -e "\033[1;33m💬 Comments\033[0m"
echo -e "\033[1;36m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m"
jira issue view "$ISSUE_KEY" --comments 50 --plain 2>/dev/null | sed -n '/^[0-9]* Comment/,$p'
