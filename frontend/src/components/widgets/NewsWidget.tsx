import { NewsFeedPanel } from '@/components/dashboard/NewsFeedPanel'
import type { WidgetProps } from '@/types/terminal'

export function NewsWidget({ ticker: _ }: WidgetProps) {
  return <NewsFeedPanel />
}
