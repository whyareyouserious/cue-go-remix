import DOMPurify from 'dompurify';
import { marked } from 'marked';

export const markdown = (text: string) => DOMPurify.sanitize(marked.parse(text, { async: false, breaks: true, gfm: true }), {
  ALLOWED_TAGS: ['p', 'br', 'hr', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'strong', 'em', 'del', 's', 'blockquote', 'pre', 'code', 'ul', 'ol', 'li', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'a'],
  ALLOWED_ATTR: ['href', 'title', 'start', 'colspan', 'rowspan'],
  ALLOW_DATA_ATTR: false,
  ALLOW_ARIA_ATTR: false,
});
