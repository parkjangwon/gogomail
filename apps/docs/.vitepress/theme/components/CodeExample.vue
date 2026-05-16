<script setup lang="ts">
import { computed, onBeforeUnmount, shallowRef } from 'vue';

type TokenKind =
  | 'plain'
  | 'method'
  | 'url'
  | 'header'
  | 'string'
  | 'key'
  | 'number'
  | 'literal'
  | 'flag'
  | 'command'
  | 'punctuation';

type CodeToken = {
  text: string;
  kind: TokenKind;
};

const props = defineProps<{
  title: string;
  language: string;
  code: string;
  copyLabel: string;
  copiedLabel: string;
}>();

const copied = shallowRef(false);
let copyTimer: number | undefined;

const highlightedLines = computed(() =>
  props.code.split(/\r?\n/).map(line => tokenizeLine(line, props.language))
);

function pushToken(tokens: CodeToken[], text: string, kind: TokenKind) {
  if (text) {
    tokens.push({ text, kind });
  }
}

function tokenizeWithPattern(
  line: string,
  pattern: RegExp,
  resolveKind: (token: string, index: number) => TokenKind,
) {
  const tokens: CodeToken[] = [];
  let cursor = 0;

  for (const match of line.matchAll(pattern)) {
    const token = match[0];
    const index = match.index ?? 0;
    pushToken(tokens, line.slice(cursor, index), 'plain');
    pushToken(tokens, token, resolveKind(token, index));
    cursor = index + token.length;
  }

  pushToken(tokens, line.slice(cursor), 'plain');
  return tokens;
}

function tokenizeJson(line: string) {
  return tokenizeWithPattern(
    line,
    /"(?:\\.|[^"\\])*"|true|false|null|-?\d+(?:\.\d+)?|[{}[\],:]/g,
    (token, index) => {
      if (/^"/.test(token)) {
        return line.slice(index + token.length).trimStart().startsWith(':') ? 'key' : 'string';
      }
      if (/^-?\d/.test(token)) {
        return 'number';
      }
      if (token === 'true' || token === 'false' || token === 'null') {
        return 'literal';
      }
      return 'punctuation';
    },
  );
}

function tokenizeShell(line: string) {
  return tokenizeWithPattern(
    line,
    /'(?:\\.|[^'\\])*'|"(?:\\.|[^"\\])*"|https?:\/\/[^\s'"]+|--?[A-Za-z0-9][A-Za-z0-9-]*|\b(?:curl|GET|POST|PUT|PATCH|DELETE|Authorization|Bearer|Content-Type)\b|\\/g,
    token => {
      if (token === 'curl') return 'command';
      if (/^(GET|POST|PUT|PATCH|DELETE)$/.test(token)) return 'method';
      if (/^https?:\/\//.test(token)) return 'url';
      if (/^--?/.test(token)) return 'flag';
      if (/^['"]/.test(token)) return 'string';
      if (token === '\\') return 'punctuation';
      return 'header';
    },
  );
}

function tokenizeHttp(line: string) {
  const requestLine = line.match(/^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)(\s+)(\S+)(\s+)(HTTP\/[\d.]+)$/);
  if (requestLine) {
    return [
      { text: requestLine[1], kind: 'method' as const },
      { text: requestLine[2], kind: 'plain' as const },
      { text: requestLine[3], kind: 'url' as const },
      { text: requestLine[4], kind: 'plain' as const },
      { text: requestLine[5], kind: 'literal' as const },
    ];
  }

  const headerLine = line.match(/^([A-Za-z0-9-]+)(:)(.*)$/);
  if (headerLine) {
    return [
      { text: headerLine[1], kind: 'header' as const },
      { text: headerLine[2], kind: 'punctuation' as const },
      ...tokenizeShell(headerLine[3]),
    ];
  }

  return tokenizeShell(line);
}

function tokenizeLine(line: string, language: string) {
  if (language === 'json') {
    return tokenizeJson(line);
  }
  if (language === 'bash' || language === 'sh' || language === 'shell') {
    return tokenizeShell(line);
  }
  if (language === 'http') {
    return tokenizeHttp(line);
  }
  return [{ text: line, kind: 'plain' as const }];
}

async function copyCode() {
  if (typeof window === 'undefined') return;

  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(props.code);
  } else {
    const textarea = document.createElement('textarea');
    textarea.value = props.code;
    textarea.setAttribute('readonly', 'true');
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.select();
    document.execCommand('copy');
    document.body.removeChild(textarea);
  }

  copied.value = true;
  if (copyTimer) {
    window.clearTimeout(copyTimer);
  }
  copyTimer = window.setTimeout(() => {
    copied.value = false;
  }, 1400);
}

onBeforeUnmount(() => {
  if (copyTimer && typeof window !== 'undefined') {
    window.clearTimeout(copyTimer);
  }
});
</script>

<template>
  <figure class="ggm-api-example">
    <figcaption class="ggm-code-header">
      <span class="ggm-code-title">{{ title }}</span>
      <button class="ggm-code-copy" type="button" @click="copyCode">
        {{ copied ? copiedLabel : copyLabel }}
      </button>
    </figcaption>
    <pre><code :class="`language-${language}`"><span
      v-for="(line, lineIndex) in highlightedLines"
      :key="`${title}-${lineIndex}`"
      class="ggm-code-line"
    ><span
      v-for="(token, tokenIndex) in line"
      :key="`${title}-${lineIndex}-${tokenIndex}`"
      :class="`ggm-token-${token.kind}`"
    >{{ token.text }}</span></span></code></pre>
  </figure>
</template>
