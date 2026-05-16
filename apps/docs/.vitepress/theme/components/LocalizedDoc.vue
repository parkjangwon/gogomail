<script setup lang="ts">
import { computed } from 'vue';
import { useData } from 'vitepress';
import { docsMessages } from '../../i18n/messages';
import { localeFromLang } from '../../i18n/terms';
import CodeExample from './CodeExample.vue';

const props = defineProps<{
  pageKey: string;
}>();

const { lang } = useData();
const locale = computed(() => localeFromLang(lang.value));
const messages = computed(() => docsMessages[locale.value]);
const page = computed(() => messages.value.pages[props.pageKey] ?? messages.value.pages.home);
const glossaryPath = computed(() => locale.value === 'en' ? '/glossary' : `/${locale.value}/glossary`);
const relatedLabel = computed(() => ({
  en: 'Related controls:',
  ko: '함께 확인할 항목:',
  ja: 'あわせて確認する項目:',
  'zh-CN': '同时确认的项目：',
})[locale.value]);
const sections = computed(() => page.value.sections.map((section, index) => ({
  ...section,
  id: section.id ?? `section-${index + 1}`,
})));
const glossaryLinks = computed(() => {
  const glossary = messages.value.pages.glossary?.sections ?? [];
  return glossary
    .flatMap(section => [section.title, ...(section.aliases ?? [])]
      .filter(Boolean)
      .map(label => ({ label, href: `${glossaryPath.value}#${section.id}` })))
    .sort((a, b) => b.label.length - a.label.length);
});

function richTextParts(text: string) {
  if (props.pageKey === 'glossary') {
    return [{ text, href: '' }];
  }

  const parts: Array<{ text: string; href: string }> = [];
  let index = 0;

  while (index < text.length) {
    const match = glossaryLinks.value.find(item => text.startsWith(item.label, index));
    if (match) {
      parts.push({ text: match.label, href: match.href });
      index += match.label.length;
      continue;
    }

    const next = index + 1;
    const last = parts[parts.length - 1];
    if (last && !last.href) {
      last.text += text.slice(index, next);
    } else {
      parts.push({ text: text.slice(index, next), href: '' });
    }
    index = next;
  }

  return parts;
}
</script>

<template>
  <div class="ggm-doc-shell">
    <article class="ggm-guide">
      <header class="ggm-doc-header">
        <p class="ggm-eyebrow">{{ page.eyebrow }}</p>
        <h1>{{ page.title }}</h1>
        <p class="ggm-lead">{{ page.lead }}</p>
      </header>

      <nav v-if="page.primaryCta || page.secondaryCta" class="ggm-doc-links" :aria-label="page.title">
        <a v-if="page.primaryCta" :href="page.primaryCta.link">
          {{ page.primaryCta.text }}
        </a>
        <a v-if="page.secondaryCta" :href="page.secondaryCta.link">
          {{ page.secondaryCta.text }}
        </a>
      </nav>

      <div class="ggm-doc-body">
        <section
          v-for="section in sections"
          :id="section.id"
          :key="section.title"
          class="ggm-doc-section"
        >
          <h2>{{ section.title }}</h2>
          <p>
            <template v-for="part in richTextParts(section.body)" :key="`${section.id}-body-${part.text}-${part.href}`">
              <a v-if="part.href" class="ggm-term-link" :href="part.href">{{ part.text }}</a>
              <span v-else>{{ part.text }}</span>
            </template>
          </p>
          <p v-for="paragraph in section.paragraphs" :key="paragraph">
            <template v-for="part in richTextParts(paragraph)" :key="`${section.id}-${paragraph}-${part.text}-${part.href}`">
              <a v-if="part.href" class="ggm-term-link" :href="part.href">{{ part.text }}</a>
              <span v-else>{{ part.text }}</span>
            </template>
          </p>
          <p v-if="section.items?.length" class="ggm-related-text">
            <strong>{{ relatedLabel }}</strong>
            {{ section.items.join(', ') }}
          </p>
          <div v-if="section.examples?.length" class="ggm-api-examples">
            <CodeExample
              v-for="example in section.examples"
              :key="`${section.id}-${example.title}`"
              :title="example.title"
              :language="example.language"
              :code="example.code"
              :copy-label="messages.code.copy"
              :copied-label="messages.code.copied"
            />
          </div>
        </section>
      </div>
    </article>

    <aside class="ggm-page-toc" :aria-label="page.title">
      <p>{{ locale === 'ko' ? '목차' : locale === 'ja' ? '目次' : locale === 'zh-CN' ? '目录' : 'On this page' }}</p>
      <a v-for="section in sections" :key="section.id" :href="`#${section.id}`">
        {{ section.title }}
      </a>
    </aside>
  </div>
</template>
