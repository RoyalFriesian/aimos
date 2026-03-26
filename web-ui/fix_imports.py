import os
import re

updates = {
    "src/app/App.tsx": [
        (r"'./components/MindmapView'", r"'./components/features/workspace/MindmapView'"),
        (r"'./components/Sidebar'", r"'./components/features/layout/Sidebar'"),
        (r"'./components/OnboardingView'", r"'./components/features/onboarding/OnboardingView'"),
    ],
    "src/app/components/features/chat/ChatPanel.tsx": [
        (r"from '\.\./types'", r"from '../../../types'"),
        (r"from '\.\./data/mockData'", r"from '../../../data/mockData'"),
        (r"from '\.\./api/client'", r"from '../../../api/client'"),
        (r"from '\./ui/", r"from '../../ui/"),
        (r"from '\./AgentDetails'", r"from './AgentDetails'"),
    ],
    "src/app/components/features/chat/AgentDetails.tsx": [
        (r"from '\.\./types'", r"from '../../../types'"),
        (r"from '\./ui/", r"from '../../ui/"),
    ],
    "src/app/components/features/workspace/MindmapView.tsx": [
        (r"from '\.\./types'", r"from '../../../types'"),
        (r"from '\.\./data/mockData'", r"from '../../../data/mockData'"),
        (r"from '\./ChatPanel'", r"from '../chat/ChatPanel'"),
        (r"from '\./ui/", r"from '../../ui/"),
        (r"from '\./ThemeProvider'", r"from '../../ThemeProvider'"),
        (r"from '\./ThemeToggle'", r"from '../../ThemeToggle'"),
    ],
    "src/app/components/features/workspace/ThreadNode.tsx": [
        (r"from '\.\./types'", r"from '../../../types'"),
        (r"from '\./ui/", r"from '../../ui/"),
        (r"from '\./ChatPanel'", r"from '../chat/ChatPanel'"),
    ],
    "src/app/components/features/layout/Sidebar.tsx": [
        (r"from '\.\./types'", r"from '../../../types'"),
        (r"from '\.\./data/mockData'", r"from '../../../data/mockData'"),
        (r"from '\./ui/", r"from '../../ui/"),
        (r"from '\./ThemeToggle'", r"from '../../ThemeToggle'"),
    ],
    "src/app/components/features/onboarding/OnboardingView.tsx": [
        (r"from '\.\./types'", r"from '../../../types'"),
        (r"from '\.\./api/client'", r"from '../../../api/client'"),
        (r"from '\./ui/", r"from '../../ui/"),
    ],
}

for filepath, replacements in updates.items():
    if os.path.exists(filepath):
        with open(filepath, 'r') as f:
            content = f.read()
        for old, new in replacements:
            content = re.sub(old, new, content)
        with open(filepath, 'w') as f:
            f.write(content)
        print(f"Updated {filepath}")
    else:
        print(f"File not found: {filepath}")
