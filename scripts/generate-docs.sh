#!/bin/bash
# Generate PDF documentation from Markdown sources using pandoc.
# Produces a complete AO documentation package for offline distribution.
#
# Usage: ./generate-docs.sh [output-dir] [version]
#
# Prerequisites:
#   - pandoc (dnf install -y pandoc or brew install pandoc)
#   - Optional: pdflatex (for PDF output; texlive-xetex on RHEL, mactex on macOS)
#
# If pandoc is not installed, outputs a list of documents that would be generated.

set -euo pipefail

OUTPUT_DIR="${1:-docs/generated}"
VERSION="${2:-dev}"
DOCS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/docs"

mkdir -p "$OUTPUT_DIR"

echo "=== Generating AO Documentation Package ==="
echo "Version: ${VERSION}"
echo "Source:  ${DOCS_DIR}"
echo "Output:  ${OUTPUT_DIR}"
echo ""

# List of documents to generate
DOCUMENTS=(
    "ao-narrative.md:System Security Plan (SSP)"
    "crypto-module-usage.md:Cryptographic Module Usage Document"
    "control-mapping.md:NIST 800-53 Control Mapping"
    "key-rotation-procedure.md:Key Rotation Procedure"
    "quic-go-crypto-audit.md:QUIC-Go Crypto Audit"
    "architecture-diagram.md:Architecture Diagram"
    "hardening-guide.md:Client Endpoint Hardening Guide"
    "monitoring-plan.md:Continuous Monitoring Plan"
    "incident-response.md:Incident Response Addendum"
)

if ! command -v pandoc >/dev/null 2>&1; then
    echo "WARNING: pandoc not found. Install it to generate PDF documents."
    echo ""
    echo "  RHEL/Fedora:  dnf install -y pandoc"
    echo "  Ubuntu/Debian: apt-get install -y pandoc"
    echo "  macOS:         brew install pandoc"
    echo ""
    echo "Documents that would be generated:"
    for doc_entry in "${DOCUMENTS[@]}"; do
        IFS=':' read -r filename title <<< "$doc_entry"
        src="${DOCS_DIR}/${filename}"
        if [ -f "$src" ]; then
            echo "  [EXISTS] ${filename} → ${filename%.md}.pdf  (${title})"
        else
            echo "  [SKIP]   ${filename} (source not found)"
        fi
    done
    echo ""
    echo "Generating Markdown index instead..."

    # Generate a combined Markdown document as fallback
    {
        echo "# cloudflared-fips AO Documentation Package"
        echo ""
        echo "**Version:** ${VERSION}"
        echo "**Generated:** $(date -u +%Y-%m-%dT%H:%M:%SZ)"
        echo ""
        echo "---"
        echo ""
        for doc_entry in "${DOCUMENTS[@]}"; do
            IFS=':' read -r filename title <<< "$doc_entry"
            src="${DOCS_DIR}/${filename}"
            if [ -f "$src" ]; then
                echo ""
                echo "# ${title}"
                echo ""
                cat "$src"
                echo ""
                echo "---"
                echo ""
            fi
        done
    } > "$OUTPUT_DIR/ao-package-${VERSION}.md"
    echo "Combined Markdown: ${OUTPUT_DIR}/ao-package-${VERSION}.md"
    exit 0
fi

echo "pandoc found: $(pandoc --version | head -1)"
echo ""

# Check for PDF engine
PDF_ENGINE=""
if command -v xelatex >/dev/null 2>&1; then
    PDF_ENGINE="--pdf-engine=xelatex"
elif command -v pdflatex >/dev/null 2>&1; then
    PDF_ENGINE="--pdf-engine=pdflatex"
else
    echo "WARNING: No LaTeX engine found. Generating HTML instead of PDF."
    echo "  Install texlive-xetex for PDF output."
    echo ""
fi

GENERATED=0
SKIPPED=0

for doc_entry in "${DOCUMENTS[@]}"; do
    IFS=':' read -r filename title <<< "$doc_entry"
    src="${DOCS_DIR}/${filename}"

    if [ ! -f "$src" ]; then
        echo "  [SKIP] ${filename} (source not found)"
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

    basename="${filename%.md}"

    if [ -n "$PDF_ENGINE" ]; then
        output="${OUTPUT_DIR}/${basename}.pdf"
        echo "  [PDF]  ${filename} → $(basename "$output")"
        pandoc "$src" \
            -o "$output" \
            --metadata title="${title}" \
            --metadata subtitle="cloudflared-fips ${VERSION}" \
            --metadata date="$(date -u +%Y-%m-%d)" \
            ${PDF_ENGINE} \
            --toc \
            --number-sections \
            -V geometry:margin=1in \
            -V mainfont="DejaVu Sans" \
            2>/dev/null || {
                echo "    PDF failed, falling back to HTML"
                output="${OUTPUT_DIR}/${basename}.html"
                pandoc "$src" -o "$output" --metadata title="${title}" --toc --standalone
            }
    else
        output="${OUTPUT_DIR}/${basename}.html"
        echo "  [HTML] ${filename} → $(basename "$output")"
        pandoc "$src" \
            -o "$output" \
            --metadata title="${title}" \
            --metadata subtitle="cloudflared-fips ${VERSION}" \
            --toc \
            --standalone \
            --number-sections \
            --css="" \
            2>/dev/null || echo "    Failed to generate $(basename "$output")"
    fi
    GENERATED=$((GENERATED + 1))
done

# Generate combined AO package (all docs in one file)
echo ""
echo "  [COMBINED] ao-package-${VERSION}..."
COMBINED_MD="${OUTPUT_DIR}/ao-package-combined.md"
{
    echo "---"
    echo "title: \"cloudflared-fips AO Documentation Package\""
    echo "subtitle: \"Version ${VERSION}\""
    echo "date: \"$(date -u +%Y-%m-%d)\""
    echo "---"
    echo ""
    for doc_entry in "${DOCUMENTS[@]}"; do
        IFS=':' read -r filename title <<< "$doc_entry"
        src="${DOCS_DIR}/${filename}"
        if [ -f "$src" ]; then
            echo ""
            # Strip any existing YAML front matter
            sed '/^---$/,/^---$/d' "$src"
            echo ""
            echo "\\pagebreak"
            echo ""
        fi
    done
} > "$COMBINED_MD"

if [ -n "$PDF_ENGINE" ]; then
    pandoc "$COMBINED_MD" \
        -o "${OUTPUT_DIR}/ao-package-${VERSION}.pdf" \
        ${PDF_ENGINE} \
        --toc \
        --number-sections \
        -V geometry:margin=1in \
        2>/dev/null && echo "    → ao-package-${VERSION}.pdf" || echo "    Combined PDF failed"
else
    pandoc "$COMBINED_MD" \
        -o "${OUTPUT_DIR}/ao-package-${VERSION}.html" \
        --toc \
        --standalone \
        --number-sections \
        2>/dev/null && echo "    → ao-package-${VERSION}.html" || echo "    Combined HTML failed"
fi

rm -f "$COMBINED_MD"

echo ""
echo "=== Documentation Generation Complete ==="
echo "  Generated: ${GENERATED} documents"
echo "  Skipped:   ${SKIPPED} (source not found)"
echo "  Output:    ${OUTPUT_DIR}/"
