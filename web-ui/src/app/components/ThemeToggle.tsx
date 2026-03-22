import { Moon, Sun } from "lucide-react"
import { useTheme } from "./ThemeProvider"

export function ThemeToggle() {
  const { theme, setTheme } = useTheme()

  const toggleTheme = () => {
    if (theme === 'light' || (theme === 'system' && !window.matchMedia("(prefers-color-scheme: dark)").matches)) {
      setTheme('dark')
    } else {
      setTheme('light')
    }
  }

  return (
    <button 
      onClick={toggleTheme} 
      className="p-3 rounded-full bg-white dark:bg-[#1e2230] border border-gray-200 dark:border-[#2a2f42] hover:bg-gray-100 dark:hover:bg-[#2a2f42] transition-colors shadow-sm focus:outline-none"
      title="Toggle theme"
    >
      <Sun className="h-5 w-5 hidden dark:block text-yellow-500" />
      <Moon className="h-5 w-5 block dark:hidden text-indigo-600" />
      <span className="sr-only">Toggle theme</span>
    </button>
  )
}