import DefaultTheme from 'vitepress/theme';
import type { Theme } from 'vitepress';
import LocalizedDoc from './components/LocalizedDoc.vue';
import './styles.css';

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('LocalizedDoc', LocalizedDoc);
  },
} satisfies Theme;
