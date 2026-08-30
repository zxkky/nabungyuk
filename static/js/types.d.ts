// Type declarations for custom window extensions used across the app
interface Window {
  escapeHTML(value: string): string;
  renderSidebar(activePage: string): void;
  checkAuth(): string | null;
  api(url: string, opts?: Record<string, unknown>): Promise<unknown>;
  formatCurrency(amount: number | string): string;
  formatDate(dateString: string): string;
  toast(message: string, type?: 'success' | 'error'): void;
  confirmAction(message: string): boolean;
  logout(): void;
  hideLoading(): void;
  showLoading(): void;
  initMobileMenu(): void;
  extractDataFromReceipt(file: File): Promise<{
    title: string;
    amount: number;
    category: string;
    date: string;
  }>;
}
