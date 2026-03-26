import os

with open('pkg/threads/store.go', 'r') as f:
    content = f.read()

content = content.replace(
    "UpdateThreadMode(threadID string, mode string) error",
    "UpdateThreadMode(threadID string, mode string) error\n\tUpdateThreadTitle(threadID string, title string) error"
)

with open('pkg/threads/store.go', 'w') as f:
    f.write(content)

print('Updated store.go')
