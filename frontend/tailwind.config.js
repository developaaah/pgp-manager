/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{html,js,svelte}'],
  theme: {
    extend: {
      fontFamily: {
        // Matches l8/layereig-ht theme
        sans:    ['SF Pro Text', 'Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        display: ['SF Pro Display', 'Inter', 'ui-sans-serif', 'system-ui', 'sans-serif'],
        mono:    ['SF Mono', 'ui-monospace', 'Menlo', 'Monaco', 'Consolas', 'monospace'],
      },

      borderRadius: {
        btn:   '7px',
        chip:  '12px',
        field: '9px',
        card:  '12px',
      },

      // Semantic tokens — auto-switch light/dark via CSS vars in app.css
      colors: {
        pgp: {
          window:   'rgb(var(--c-window))',
          sidebar:  'rgb(var(--c-sidebar))',
          titlebar: 'rgb(var(--c-titlebar))',
          elevated: 'rgb(var(--c-elevated))',

          border:       'rgb(var(--c-border))',
          'border-strong': 'rgb(var(--c-border-strong))',

          text:   'rgb(var(--c-text))',
          'text-2': 'rgb(var(--c-text-2))',
          'text-3': 'rgb(var(--c-text-3))',
          'text-4': 'rgb(var(--c-text-4))',

          field:        'rgb(var(--c-field))',
          'field-border': 'rgb(var(--c-field-border))',

          fill:   'rgb(var(--c-fill))',
          'fill-2': 'rgb(var(--c-fill-2))',

          accent:    'rgb(var(--c-accent))',
          'accent-2': 'rgb(var(--c-accent-2))',
          'accent-bg': 'rgb(var(--c-accent-bg))',

          'nav-text':         'rgb(var(--c-nav-text))',
          'nav-label':        'rgb(var(--c-nav-label))',
          'nav-active-bg':    'rgb(var(--c-nav-active-bg))',
          'nav-active-text':  'rgb(var(--c-nav-active-text))',
          'nav-hover-bg':     'rgb(var(--c-nav-hover-bg))',
          'nav-hover-text':   'rgb(var(--c-nav-hover-text))',
        },
      },
    },
  },
  plugins: [],
}
