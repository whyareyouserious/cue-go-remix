declare module '*wails/runtime.js' {
  export const Call: { ByName(name: string, ...args: unknown[]): Promise<any> };
  export const Events: { On(name: string, callback: (event: { data: any }) => void): () => void };
  export const Clipboard: { SetText(text: string): Promise<void> };
  export const Browser: { OpenURL(url: string): Promise<void> };
}
