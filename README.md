cat << 'EOF' > .gitignore
# Скомпилированные бинарники
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
/main

# Файлы с секретами и конфигурацией среды
.env

# Файлы IDE и редакторов
.idea/
.vscode/
*.swp

# Системные файлы
.DS_Store
Thumbs.db
EOF
