

export const $ = <T extends HTMLElement = HTMLElement>(selector: string, root: ParentNode = document): T => {
  const node = root.querySelector<T>(selector);
  if (!node)
    throw new Error(`界面元素不存在：${selector}`);
  return node;
};
