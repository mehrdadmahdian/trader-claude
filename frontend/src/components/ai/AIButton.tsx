import { Sparkles } from 'lucide-react'

interface AIButtonProps {
  onClick: () => void
  isOpen: boolean
}

export function AIButton({ onClick, isOpen }: AIButtonProps) {
  return (
    <button
      onClick={onClick}
      className={`fixed bottom-6 right-6 z-50 flex items-center justify-center w-12 h-12 rounded-full shadow-lg transition-all duration-200 ${
        isOpen
          ? 'bg-violet-500 hover:bg-violet-600 text-white rotate-90'
          : 'bg-violet-600 hover:bg-violet-700 text-white'
      }`}
      aria-label="AI Assistant"
      title="AI Assistant"
    >
      <Sparkles className="h-5 w-5" />
    </button>
  )
}
