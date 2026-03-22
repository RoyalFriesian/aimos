with open('../web-ui/src/app/App.tsx', 'r') as f:
    text = f.read()

import re
import sys

# find the split point where it duplicates
# we can just use regex to replace two occurrences of the block with one.
block = r"""  const \[isLoadingProject, setIsLoadingProject\] = useState\(false\);.*?finally \{\s*?setIsLoadingProject\(false\);\s*?\}\s*?\};"""

new_text = re.sub(block, "", text, count=1, flags=re.DOTALL)

with open('../web-ui/src/app/App.tsx', 'w') as f:
    f.write(new_text)
