---
name: alt-text
description: Guide for writing descriptive, accessible alternative text for images following Section 508 and WCAG standards. Use when Claude needs to create alt text for web content, documents, or any digital images that require accessibility compliance.
---

# Alt Text Writing Guide

## Overview

Alternative text (alt text) provides text equivalents for images, making visual content accessible to people who are blind, have low vision, or have difficulty processing visual information. Alt text is read by screen readers, displayed when images fail to load, and indexed by search engines.

**Core principle**: Alt text should convey the meaning and purpose of an image in its context, not simply describe what's visible.

## When Alt Text is Required

### Images that NEED alt text:
- **Informative images**: Convey concepts, ideas, or information
- **Functional images**: Buttons, links, icons that perform actions
- **Images of text**: Any text rendered as an image
- **Complex images**: Charts, graphs, diagrams, maps
- **Images in forms**: Input buttons, image maps

### Images that should have EMPTY alt text (`alt=""`):
- **Decorative images**: Purely aesthetic with no informational value
- **Spacer images**: Used for layout purposes
- **Images with adjacent text**: When surrounding text fully describes the image
- **Images within linked content**: When link text already describes the destination

**Never omit the alt attribute entirely** - use `alt=""` for decorative images.

## Core Writing Principles

### 1. Be Accurate and Equivalent
- Convey the same information the image would convey to a sighted user
- Include all relevant information visible in the image
- Don't add information not present in the image
- Don't make assumptions or add interpretation unless contextually necessary

### 2. Be Succinct
- Aim for **under 150 characters** when possible (screen reader comfort zone)
- Prioritize the most important information first
- For complex images requiring longer descriptions, use alt text for summary + provide long description elsewhere
- Every word should serve a purpose

### 3. Consider Context and Purpose
- The same image may need different alt text in different contexts
- Ask: "What would I say to describe this image over the phone?"
- Consider why the image is included and what it's meant to communicate
- Match the tone and style of surrounding content

### 4. Avoid Redundancy
- **Don't use**: "image of", "picture of", "graphic of", "photo of"
- Screen readers already announce "image" - this is redundant
- Exception: When the type matters (e.g., "screenshot of" or "diagram showing")

### 5. Don't Include File Names or Technical Details
- **Don't use**: "IMG_5324.jpg" or "banner_v2_final.png"
- Focus on content and meaning, not technical metadata

## Guidelines by Image Type

### Informative Images

Convey a simple concept or information.

**Good**: "Woman presenting to a group of colleagues in a conference room"
**Bad**: "Image of business meeting"
**Why**: The good version provides specific, useful context

**Good**: "Warning: Slippery floor"
**Bad**: "Yellow caution sign"
**Why**: The meaning (warning message) is more important than appearance

### Functional Images

Images used as links or buttons.

**Good**: "Search" (for a magnifying glass icon button)
**Bad**: "Magnifying glass" 
**Why**: Describes the function, not the visual

**Good**: "Download PDF" (for a download icon)
**Bad**: "Down arrow icon"
**Why**: Tells users what will happen when clicked

### Images of Text

Text rendered as an image must be included in alt text.

**Good**: "Community Meeting - Tuesday, March 15 at 6 PM - City Hall Auditorium"
**Bad**: "Event flyer"
**Why**: All text content must be accessible

**Note**: For lengthy text in images, provide the complete text in a long description and use alt text for a brief summary.

### Decorative Images

No informational value - use empty alt text.

```html
<img src="decorative-swirl.png" alt="">
```

**Examples of decorative images**:
- Background patterns or textures
- Purely aesthetic elements
- Visual separators (borders, dividers)
- Images that duplicate adjacent text content

### Complex Images

Charts, graphs, diagrams, maps require two-level descriptions.

**Alt text** (brief summary):
"Bar chart showing website traffic by month for 2024"

**Long description** (detailed, separate from alt):
"Website traffic started at 50,000 visits in January, peaked at 120,000 in June, and ended at 95,000 in December. The summer months showed the highest traffic."

**Techniques for long descriptions**:
- Use `aria-describedby` to reference detailed description
- Provide data table alongside chart
- Link to separate page with full description
- Use `<details>` element for expandable description

### Logos and Brand Images

Include the organization name.

**Good**: "Acme Corporation logo"
**Bad**: "Company logo"
**Why**: Identifies the specific organization

**Good**: "National Park Service arrowhead emblem"
**Bad**: "Logo"
**Why**: More descriptive for recognition

### People and Portraits

Describe relevant identifying information based on context.

**Professional context**:
"Dr. Sarah Johnson, Chief Technology Officer"

**Event context**:
"Five team members celebrating project completion with champagne"

**Historical context**:
"President Franklin D. Roosevelt signing legislation in 1935"

**What to include**: Names (if relevant), roles, actions, setting
**What to avoid**: Subjective descriptions of appearance unless contextually relevant

## Common Mistakes to Avoid

### ❌ Being Too Vague
**Bad**: "Document"
**Good**: "Project timeline showing milestones from January to June"

### ❌ Being Too Verbose
**Bad**: "This is a photograph taken outdoors on what appears to be a sunny day showing approximately seven or eight people who seem to be..."
**Good**: "Team members collaborating at an outdoor workshop"

### ❌ Using Placeholder Text
**Bad**: "image", "photo", "[image]", "untitled"
**Good**: Actual description of the image content

### ❌ Starting with Redundant Phrases
**Bad**: "A picture of a dog playing in the park"
**Good**: "Dog playing in the park"

### ❌ Including Subjective Opinions
**Bad**: "Beautiful sunset over the ocean"
**Good**: "Sunset over the ocean with orange and purple clouds"
(Exception: In artistic/editorial contexts where mood matters)

### ❌ Describing Images Out of Context
The same image needs different alt text based on its purpose:

**In a vacation blog**: "Golden Gate Bridge at sunrise"
**In a structural engineering article**: "Golden Gate Bridge showing suspension cable configuration"
**In a travel guide**: "Golden Gate Bridge, San Francisco's most recognizable landmark"

## Testing Your Alt Text

Ask yourself:
1. **Equivalence**: Would someone listening to this alt text get the same information as someone viewing the image?
2. **Brevity**: Have I eliminated all unnecessary words?
3. **Context**: Does this alt text make sense alongside the surrounding content?
4. **Independence**: Would this make sense if the image failed to load?
5. **Neutrality**: Have I avoided unnecessary opinions or interpretations?

## Quick Decision Tree

```
Is the image decorative only?
├─ YES → alt=""
└─ NO → Does the image convey information?
    ├─ YES → Write descriptive alt text
    │   ├─ Simple image → Brief description (under 150 chars)
    │   └─ Complex image → Brief alt + long description
    └─ NO → Is it functional (button, link)?
        ├─ YES → Describe the function/action
        └─ NO → Consider if it's truly needed
```

## Examples by Scenario

### E-commerce Product
**Image**: Running shoe product photo
**Good**: "Nike Air Zoom Pegasus 40 running shoe in blue and white"
**Context**: Helps users identify the specific product

### Data Visualization
**Image**: Pie chart
**Alt**: "Pie chart of 2024 budget allocation"
**Long description**: "Budget allocation: Operations 40%, Personnel 35%, Equipment 15%, Training 10%"

### Social Media
**Image**: Cat photo
**Context matters**:
- Personal post: "My cat Oliver sleeping in a sunbeam"
- Adoption site: "Orange tabby cat, 2 years old, friendly with children"
- Meme: "Cat looking skeptical at laptop screen"

### Instructional Content
**Image**: Screenshot showing software steps
**Good**: "Screenshot showing File menu with Save As option highlighted"
**Bad**: "Computer screen"

### Historical Document
**Image**: Historical photograph
**Good**: "Women working in a munitions factory during World War II, circa 1943"
**Bad**: "Old black and white photo"

## Special Considerations

### Mathematical or Scientific Images
Include formula or notation:
"Einstein's equation: E equals m c squared"

### Multilingual Content
Alt text should match the language of the surrounding content.

### Animated GIFs
Describe the key action:
"Animated diagram showing how a piston engine operates"

### Image Maps
Each clickable region needs its own alt text describing its function.

## Tools and Validation

While writing alt text, remember:
- Screen reader users navigate by heading, links, and images
- Alt text should make sense in linear reading order
- Test with actual screen readers when possible (NVDA, JAWS, VoiceOver)
- Validate HTML to ensure alt attributes are present
- Review with users who rely on assistive technology

## Quick Reference Table

| Image Type | Alt Text Approach | Example |
|------------|------------------|---------|
| Decorative | Empty alt (`alt=""`) | Background pattern |
| Informative | Brief description | "Solar panel installation on residential roof" |
| Functional | Describe action | "Submit form" |
| Text image | Include all text | "Now Hiring - Apply Today" |
| Complex | Brief alt + long description | Alt: "Organizational chart" + detailed hierarchy |
| Logo | Organization name | "World Health Organization logo" |
| Icon button | Button function | "Close dialog" |

## Key Takeaway

**Write alt text for understanding, not description.** The goal is equivalence - ensuring that people who cannot see the image receive the same information, functionality, and experience as those who can.

## Sources

This guide is based on best practices and guidance from:

1. **U.S. General Services Administration.** "What is Alternative Text?" Section508.gov.  
   https://www.section508.gov/training/alt-text/what-is-alternative-text/

2. **U.S. General Services Administration.** "Create Accessible Digital Products: Alternative Text." Section508.gov.  
   https://www.section508.gov/create/alternative-text/

3. **U.S. Department of Veterans Affairs.** "Alt Text." Digital.va.gov Accessibility Guide.  
   https://digital.va.gov/accessibility/getting-started/alt-text/

4. **Colorado Governor's Office of Information Technology.** "A Guide to Accessible Web Services: Accessible Images." OIT.Colorado.gov.  
   https://oit.colorado.gov/standards-policies-guides/a-guide-to-accessible-web-services/accessible-images
