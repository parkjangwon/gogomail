# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: verify-i18n.spec.ts >> Verify Korean i18n translations on Dashboard
- Location: e2e/verify-i18n.spec.ts:3:1

# Error details

```
Error: expect(received).toContain(expected) // indexOf

Expected substring: "대시보드"
Received string:    "(self.__next_f=self.__next_f||[]).push([0])self.__next_f.push([1,\"6:I[\\\"(app-pages-browser)/./node_modules/.pnpm/next@15.5.18_@babel+core@7.29.0_@playwright+test@1.59.1_react-dom@19.2.6_react@19.2.6__react@19.2.6/node_modules/next/dist/next-devtools/userspace/app/segment-explorer-node.js\\\",[\\\"app-pages-internals\\\",\\\"static/chunks/app-pages-internals.js\\\"],\\\"SegmentViewNode\\\"]\\n8:\\\"$Sreact.fragment\\\"\\n17:I[\\\"(app-pages-browser)/./src/app/providers.tsx\\\",[\\\"app/layout\\\",\\\"static/chunks/app/layout.js\\\"],\\\"Providers\\\"]\\n19:I[\\\"(app-pages-browser)/./node_modules/.pnpm/next@15.5.18_@babel+core@7.29.0_@playwright+test@1.59.1_react-dom@19.2.6_react@19.2.6__react@19.2.6/node_modules/next/dist/client/components/layout-router.js\\\",[\\\"app-pages-internals\\\",\\\"static/chunks/app-pages-internals.js\\\"],\\\"\\\"]\\n1b:I[\\\"(app-pages-browser)/./node_modules/.pnpm/next@15.5.18_@babel+core@7.29.0_@playwright+test@1.59.1_react-dom@19.2.6_react@19.2.6__react@19.2.6/node_modules/next/dist/client/components/render-from-template-context.js\\\",[\\\"app-pages-internals\\\",\\\"static/chunks/app-pages-internals.js\\\"],\\\"\\\"]\\n2f:I[\\\"(app-pages-browser)/./node_modules/.pnpm/next@15.5.18_@babel+core@7.29.0_@playwright+test@1.59.1_react-dom@19.2.6_react@19.2.6__react@19.2.6/node_modules/next/dist/client/components/client-page.js\\\",[\\\"app-pages-internals\\\",\\\"static/chunks/app-pages-internals.js\\\"],\\\"ClientPageRoot\\\"]\\n30:I[\\\"(app-pages-browser)/./src/app/login/page.tsx\\\",[\\\"app/login/page\\\",\\\"static/chunks/app/login/page.js\\\"],\\\"default\\\"]\\n33:I[\\\"(app-pages-browser)/./node_modules/.pnpm/next@15.5.18_@babel+core@7.29.0_@playwright+test@1.59.1_react-dom@19.2.6_react@19.2.6__react@19.2.6/node_modules/next/dist/lib/framework/boundary-components.js\\\",[\\\"app-pages-internals\\\",\\\"static/chunks/app-pages-internals.js\\\"],\\\"OutletBoundary\\\"]\\n3a:I[\\\"(app-pages-browser)/./node_modules/.pnpm/next@15.5.18_@babel+core@7.29.0_@playwright+test@1.59.1_react-dom@19.2.6_react@19.2.6__react@19.2.6/node_modules/next/dist/client/components/metadata/async-metadata.js\\\",[\\\"app-pages-internals\\\",\\\"static/chunks/app-pages-internals.js\\\"],\\\"AsyncMetadataOutlet\\\"]\\n42:I[\\\"(app-pages-browser)/./node_modules/.pnpm/next@15.5.18_\"])self.__next_f.push([1,\"@babel+core@7.29.0_@playwright+test@1.59.1_react-dom@19.2.6_react@19.2.6__react@19.2.6/node_modules/next/dist/lib/framework/boundary-components.js\\\",[\\\"app-pages-internals\\\",\\\"static/chunks/app-pages-internals.js\\\"],\\\"ViewportBoundary\\\"]\\n48:I[\\\"(app-pages-browser)/./node_modules/.pnpm/next@15.5.18_@babel+core@7.29.0_@playwright+test@1.59.1_react-dom@19.2.6_react@19.2.6__react@19.2.6/node_modules/next/dist/lib/framework/boundary-components.js\\\",[\\\"app-pages-internals\\\",\\\"static/chunks/app-pages-internals.js\\\"],\\\"MetadataBoundary\\\"]\\n4d:\\\"$Sreact.suspense\\\"\\n51:I[\\\"(app-pages-browser)/./node_modules/.pnpm/next@15.5.18_@babel+core@7.29.0_@playwright+test@1.59.1_react-dom@19.2.6_react@19.2.6__react@19.2.6/node_modules/next/dist/client/components/builtin/global-error.js\\\",[\\\"app-pages-internals\\\",\\\"static/chunks/app-pages-internals.js\\\"],\\\"\\\"]\\n59:I[\\\"(app-pages-browser)/./node_modules/.pnpm/next@15.5.18_@babel+core@7.29.0_@playwright+test@1.59.1_react-dom@19.2.6_react@19.2.6__react@19.2.6/node_modules/next/dist/lib/metadata/generate/icon-mark.js\\\",[\\\"app-pages-internals\\\",\\\"static/chunks/app-pages-internals.js\\\"],\\\"IconMark\\\"]\\n:HL[\\\"/_next/static/css/app/layout.css?v=1778398178793\\\",\\\"style\\\"]\\n:HL[\\\"/_next/static/css/app/login/page.css?v=1778398178793\\\",\\\"style\\\"]\\n:N1778398178755.9128\\n3:\\\"$EObject.defineProperty(()=\\u003e{ctx.componentMod.preloadStyle(fullHref,ctx.renderOpts.crossOrigin,ctx.nonce)},\\\\\\\"name\\\\\\\",{value:\\\\\\\"\\\\\\\"})\\\"\\n4:\\\"$EObject.defineProperty(()=\\u003e{ctx.componentMod.preloadStyle(fullHref,ctx.renderOpts.crossOrigin,ctx.nonce)},\\\\\\\"name\\\\\\\",{value:\\\\\\\"\\\\\\\"})\\\"\\n2:{\\\"name\\\":\\\"Preloads\\\",\\\"key\\\":null,\\\"env\\\":\\\"Server\\\",\\\"stack\\\":[],\\\"props\\\":{\\\"preloadCallbacks\\\":[\\\"$3\\\",\\\"$4\\\"]}}\\n5:[]\\n7:[]\\n9:[[\\\"Array.map\\\",\\\"\\\",0,0,0,0,false]]\\nc:I[\\\"(app-pages-browser)/./node_modules/.pnpm/next@15.5.18_@babel+core@7.29.0_@playwright+test@1.59.1_react-dom@19.2.6_react@19.2.6__react@19.2.6/node_modules/next/dist/client/components/layout-router.js\\\",[\\\"app-pages-internals\\\",\\\"static/chunks/app-pages-internals.js\\\"],\\\"\\\"]\\nf:I[\\\"(app-pages-browser)/./node_modules/.pnpm/next@15.5.18_@babel+core@7.29.0_@playwright+test@1.59.1_rea\"])self.__next_f.push([1,\"ct-dom@19.2.6_react@19.2.6__react@19.2.6/node_modules/next/dist/client/components/render-from-template-context.js\\\",[\\\"app-pages-internals\\\",\\\"static/chunks/app-pages-internals.js\\\"],\\\"\\\"]\\n10:{}\\n11:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\ne:{\\\"children\\\":[\\\"$\\\",\\\"$Lf\\\",null,\\\"$10\\\",null,\\\"$11\\\",1]}\\n12:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\nd:{\\\"parallelRouterKey\\\":\\\"children\\\",\\\"error\\\":\\\"$undefined\\\",\\\"errorStyles\\\":\\\"$undefined\\\",\\\"errorScripts\\\":\\\"$undefined\\\",\\\"template\\\":[\\\"$\\\",\\\"$8\\\",null,\\\"$e\\\",null,\\\"$12\\\",0],\\\"templateStyles\\\":\\\"$undefined\\\",\\\"templateScripts\\\":\\\"$undefined\\\",\\\"notFound\\\":\\\"$Y\\\",\\\"forbidden\\\":\\\"$undefined\\\",\\\"unauthorized\\\":\\\"$undefined\\\",\\\"segmentViewBoundaries\\\":\\\"$Y\\\"}\\n13:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\nb:{\\\"name\\\":\\\"RootLayout\\\",\\\"key\\\":null,\\\"env\\\":\\\"Server\\\",\\\"stack\\\":[],\\\"props\\\":{\\\"children\\\":[\\\"$\\\",\\\"$Lc\\\",null,\\\"$d\\\",null,\\\"$13\\\",1],\\\"params\\\":\\\"$Y\\\"}}\\n14:[[\\\"RootLayout\\\",\\\"webpack-internal:///(rsc)/./src/app/layout.tsx\\\",23,87,22,1,false]]\\n15:[[\\\"RootLayout\\\",\\\"webpack-internal:///(rsc)/./src/app/layout.tsx\\\",26,94,22,1,false]]\\n16:[[\\\"RootLayout\\\",\\\"webpack-internal:///(rsc)/./src/app/layout.tsx\\\",27,98,22,1,false]]\\n18:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\n1a:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\n1c:[]\\n1e:{\\\"name\\\":\\\"NotFound\\\",\\\"key\\\":null,\\\"env\\\":\\\"Server\\\",\\\"stack\\\":[],\\\"props\\\":{}}\\n1f:{\\\"name\\\":\\\"HTTPAccessErrorFallback\\\",\\\"key\\\":null,\\\"env\\\":\\\"Server\\\",\\\"owner\\\":\\\"$1e\\\",\\\"stack\\\":[],\\\"props\\\":{\\\"status\\\":404,\\\"message\\\":\\\"This page could not be found.\\\"}}\\n20:[]\\n21:[]\\n22:[]\\n23:[]\\n24:[]\\n25:[]\\n26:[]\\n27:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\n28:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\n29:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\n2a:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true],[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\n2b:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true],[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\n2c:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true],[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\n2d:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true],[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\n2e:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true],[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\n31:[[\\\"Array.map\\\",\\\"\\\",0,0,0,0,false],[\\\"Array.map\\\",\\\"\\\",0,0,0,0,false],[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\n32:[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true],[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]]\\n36:\\\"$EObject.defineProp\"])self.__next_f.push([1,\"erty(async function getViewportReady() {\\\\n        await viewport();\\\\n        return undefined;\\\\n    },\\\\\\\"name\\\\\\\",{value:\\\\\\\"getViewportReady\\\\\\\"})\\\"\\n35:{\\\"name\\\":\\\"__next_outlet_boundary__\\\",\\\"key\\\":null,\\\"env\\\":\\\"Server\\\",\\\"stack\\\":[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true],[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]],\\\"props\\\":{\\\"ready\\\":\\\"$36\\\"}}\\n38:{\\\"name\\\":\\\"StreamingMetadataOutletImpl\\\",\\\"key\\\":null,\\\"env\\\":\\\"Server\\\",\\\"stack\\\":[[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true],[\\\"Promise.all\\\",\\\"\\\",0,0,0,0,true]],\\\"props\\\":{}}\\n39:[]\\n3c:[]\\n3e:{\\\"name\\\":\\\"NonIndex\\\",\\\"key\\\":null,\\\"env\\\":\\\"Server\\\",\\\"stack\\\":[],\\\"props\\\":{\\\"pagePath\\\":\\\"/login\\\",\\\"statusCode\\\":200,\\\"isPossibleServerAction\\\":false}}\\n40:{\\\"name\\\":\\\"ViewportTree\\\",\\\"key\\\":null,\\\"env\\\":\\\"Server\\\",\\\"stack\\\":[],\\\"props\\\":{}}\\n41:[]\\n44:{\\\"name\\\":\\\"__next_viewport_boundary__\\\",\\\"key\\\":null,\\\"env\\\":\\\"Server\\\",\\\"owner\\\":\\\"$40\\\",\\\"stack\\\":[],\\\"props\\\":{}}\\n46:{\\\"name\\\":\\\"MetadataTree\\\",\\\"key\\\":null,\\\"env\\\":\\\"Server\\\",\\\"stack\\\":[],\\\"props\\\":{}}\\n47:[]\\n4a:{\\\"name\\\":\\\"__next_metadata_boundary__\\\",\\\"key\\\":null,\\\"env\\\":\\\"Server\\\",\\\"owner\\\":\\\"$46\\\",\\\"stack\\\":[],\\\"props\\\":{}}\\n4b:[]\\n4c:[]\\n4f:{\\\"name\\\":\\\"MetadataResolver\\\",\\\"key\\\":null,\\\"env\\\":\\\"Server\\\",\\\"owner\\\":\\\"$4a\\\",\\\"stack\\\":[],\\\"props\\\":{}}\\n52:[]\\n53:[]\\n54:[]\\n55:[]\\n56:[]\\n57:[[\\\"Array.map\\\",\\\"\\\",0,0,0,0,false]]\\n58:[]\\n1:D\\\"$2\\\"\\n1:null\\na:D\\\"$b\\\"\\n1d:D\\\"$1e\\\"\\n1d:D\\\"$1f\\\"\\n\"])self.__next_f.push([1,\"1d:[[\\\"$\\\",\\\"title\\\",null,{\\\"children\\\":\\\"404: This page could not be found.\\\"},\\\"$1f\\\",\\\"$20\\\",1],[\\\"$\\\",\\\"div\\\",null,{\\\"style\\\":{\\\"fontFamily\\\":\\\"system-ui,\\\\\\\"Segoe UI\\\\\\\",Roboto,Helvetica,Arial,sans-serif,\\\\\\\"Apple Color Emoji\\\\\\\",\\\\\\\"Segoe UI Emoji\\\\\\\"\\\",\\\"height\\\":\\\"100vh\\\",\\\"textAlign\\\":\\\"center\\\",\\\"display\\\":\\\"flex\\\",\\\"flexDirection\\\":\\\"column\\\",\\\"alignItems\\\":\\\"center\\\",\\\"justifyContent\\\":\\\"center\\\"},\\\"children\\\":[\\\"$\\\",\\\"div\\\",null,{\\\"children\\\":[[\\\"$\\\",\\\"style\\\",null,{\\\"dangerouslySetInnerHTML\\\":{\\\"__html\\\":\\\"body{color:#000;background:#fff;margin:0}.next-error-h1{border-right:1px solid rgba(0,0,0,.3)}@media (prefers-color-scheme:dark){body{color:#fff;background:#000}.next-error-h1{border-right:1px solid rgba(255,255,255,.3)}}\\\"}},\\\"$1f\\\",\\\"$23\\\",1],[\\\"$\\\",\\\"h1\\\",null,{\\\"className\\\":\\\"next-error-h1\\\",\\\"style\\\":{\\\"display\\\":\\\"inline-block\\\",\\\"margin\\\":\\\"0 20px 0 0\\\",\\\"padding\\\":\\\"0 23px 0 0\\\",\\\"fontSize\\\":24,\\\"fontWeight\\\":500,\\\"verticalAlign\\\":\\\"top\\\",\\\"lineHeight\\\":\\\"49px\\\"},\\\"children\\\":404},\\\"$1f\\\",\\\"$24\\\",1],[\\\"$\\\",\\\"div\\\",null,{\\\"style\\\":{\\\"display\\\":\\\"inline-block\\\"},\\\"children\\\":[\\\"$\\\",\\\"h2\\\",null,{\\\"style\\\":{\\\"fontSize\\\":14,\\\"fontWeight\\\":400,\\\"lineHeight\\\":\\\"49px\\\",\\\"margin\\\":0},\\\"children\\\":\\\"This page could not be found.\\\"},\\\"$1f\\\",\\\"$26\\\",1]},\\\"$1f\\\",\\\"$25\\\",1]]},\\\"$1f\\\",\\\"$22\\\",1]},\\\"$1f\\\",\\\"$21\\\",1]]\\n\"])self.__next_f.push([1,\"a:[\\\"$\\\",\\\"html\\\",null,{\\\"lang\\\":\\\"ko\\\",\\\"suppressHydrationWarning\\\":true,\\\"children\\\":[\\\"$\\\",\\\"body\\\",null,{\\\"children\\\":[\\\"$\\\",\\\"$L17\\\",null,{\\\"children\\\":[\\\"$\\\",\\\"$L19\\\",null,{\\\"parallelRouterKey\\\":\\\"children\\\",\\\"error\\\":\\\"$undefined\\\",\\\"errorStyles\\\":\\\"$undefined\\\",\\\"errorScripts\\\":\\\"$undefined\\\",\\\"template\\\":[\\\"$\\\",\\\"$L1b\\\",null,{},null,\\\"$1a\\\",1],\\\"templateStyles\\\":\\\"$undefined\\\",\\\"templateScripts\\\":\\\"$undefined\\\",\\\"notFound\\\":[\\\"$\\\",\\\"$L6\\\",\\\"c-not-found\\\",{\\\"type\\\":\\\"not-found\\\",\\\"pagePath\\\":\\\"__next_builtin__not-found.js\\\",\\\"children\\\":[\\\"$1d\\\",[]]},null,\\\"$1c\\\",0],\\\"forbidden\\\":\\\"$undefined\\\",\\\"unauthorized\\\":\\\"$undefined\\\",\\\"segmentViewBoundaries\\\":[[\\\"$\\\",\\\"$L6\\\",null,{\\\"type\\\":\\\"boundary:not-found\\\",\\\"pagePath\\\":\\\"__next_builtin__not-found.js@boundary\\\"},null,\\\"$27\\\",1],\\\"$undefined\\\",\\\"$undefined\\\",[\\\"$\\\",\\\"$L6\\\",null,{\\\"type\\\":\\\"boundary:global-error\\\",\\\"pagePath\\\":\\\"__next_builtin__global-error.js\\\"},null,\\\"$28\\\",1]]},null,\\\"$18\\\",1]},\\\"$b\\\",\\\"$16\\\",1]},\\\"$b\\\",\\\"$15\\\",1]},\\\"$b\\\",\\\"$14\\\",1]\\n\"])self.__next_f.push([1,\"34:D\\\"$35\\\"\\n37:D\\\"$38\\\"\\n37:[\\\"$\\\",\\\"$L3a\\\",null,{\\\"promise\\\":\\\"$@3b\\\"},\\\"$38\\\",\\\"$39\\\",1]\\n3d:D\\\"$3e\\\"\\n3d:null\\n3f:D\\\"$40\\\"\\n43:D\\\"$44\\\"\\n3f:[[\\\"$\\\",\\\"$L42\\\",null,{\\\"children\\\":\\\"$L43\\\"},\\\"$40\\\",\\\"$41\\\",1],null]\\n45:D\\\"$46\\\"\\n49:D\\\"$4a\\\"\\n4e:D\\\"$4f\\\"\\n49:[\\\"$\\\",\\\"div\\\",null,{\\\"hidden\\\":true,\\\"children\\\":[\\\"$\\\",\\\"$4d\\\",null,{\\\"fallback\\\":null,\\\"children\\\":\\\"$L4e\\\"},\\\"$4a\\\",\\\"$4c\\\",1]},\\\"$4a\\\",\\\"$4b\\\",1]\\n45:[\\\"$\\\",\\\"$L48\\\",null,{\\\"children\\\":\\\"$49\\\"},\\\"$46\\\",\\\"$47\\\",1]\\n50:[]\\n\"])self.__next_f.push([1,\"0:{\\\"P\\\":\\\"$1\\\",\\\"b\\\":\\\"development\\\",\\\"p\\\":\\\"\\\",\\\"c\\\":[\\\"\\\",\\\"login\\\"],\\\"i\\\":false,\\\"f\\\":[[[\\\"\\\",{\\\"children\\\":[\\\"login\\\",{\\\"children\\\":[\\\"__PAGE__\\\",{}]}]},\\\"$undefined\\\",\\\"$undefined\\\",true],[\\\"\\\",[\\\"$\\\",\\\"$L6\\\",\\\"layout\\\",{\\\"type\\\":\\\"layout\\\",\\\"pagePath\\\":\\\"layout.tsx\\\",\\\"children\\\":[\\\"$\\\",\\\"$8\\\",\\\"c\\\",{\\\"children\\\":[[[\\\"$\\\",\\\"link\\\",\\\"0\\\",{\\\"rel\\\":\\\"stylesheet\\\",\\\"href\\\":\\\"/_next/static/css/app/layout.css?v=1778398178793\\\",\\\"precedence\\\":\\\"next_static/css/app/layout.css\\\",\\\"crossOrigin\\\":\\\"$undefined\\\",\\\"nonce\\\":\\\"$undefined\\\"},null,\\\"$9\\\",0]],\\\"$a\\\"]},null,\\\"$7\\\",1]},null,\\\"$5\\\",0],{\\\"children\\\":[\\\"login\\\",[\\\"$\\\",\\\"$8\\\",\\\"c\\\",{\\\"children\\\":[null,[\\\"$\\\",\\\"$L19\\\",null,{\\\"parallelRouterKey\\\":\\\"children\\\",\\\"error\\\":\\\"$undefined\\\",\\\"errorStyles\\\":\\\"$undefined\\\",\\\"errorScripts\\\":\\\"$undefined\\\",\\\"template\\\":[\\\"$\\\",\\\"$L1b\\\",null,{},null,\\\"$2b\\\",1],\\\"templateStyles\\\":\\\"$undefined\\\",\\\"templateScripts\\\":\\\"$undefined\\\",\\\"notFound\\\":\\\"$undefined\\\",\\\"forbidden\\\":\\\"$undefined\\\",\\\"unauthorized\\\":\\\"$undefined\\\",\\\"segmentViewBoundaries\\\":[\\\"$undefined\\\",\\\"$undefined\\\",\\\"$undefined\\\",\\\"$undefined\\\"]},null,\\\"$2a\\\",1]]},null,\\\"$29\\\",0],{\\\"children\\\":[\\\"__PAGE__\\\",[\\\"$\\\",\\\"$8\\\",\\\"c\\\",{\\\"children\\\":[[\\\"$\\\",\\\"$L6\\\",\\\"c-page\\\",{\\\"type\\\":\\\"page\\\",\\\"pagePath\\\":\\\"login/page.tsx\\\",\\\"children\\\":[\\\"$\\\",\\\"$L2f\\\",null,{\\\"Component\\\":\\\"$30\\\",\\\"searchParams\\\":{},\\\"params\\\":{}},null,\\\"$2e\\\",1]},null,\\\"$2d\\\",1],[[\\\"$\\\",\\\"link\\\",\\\"0\\\",{\\\"rel\\\":\\\"stylesheet\\\",\\\"href\\\":\\\"/_next/static/css/app/login/page.css?v=1778398178793\\\",\\\"precedence\\\":\\\"next_static/css/app/login/page.css\\\",\\\"crossOrigin\\\":\\\"$undefined\\\",\\\"nonce\\\":\\\"$undefined\\\"},null,\\\"$31\\\",0]],[\\\"$\\\",\\\"$L33\\\",null,{\\\"children\\\":[\\\"$L34\\\",\\\"$37\\\"]},null,\\\"$32\\\",1]]},null,\\\"$2c\\\",0],{},null,false]},null,false]},null,false],[\\\"$\\\",\\\"$8\\\",\\\"h\\\",{\\\"children\\\":[\\\"$3d\\\",\\\"$3f\\\",\\\"$45\\\"]},null,\\\"$3c\\\",0],false]],\\\"m\\\":\\\"$W50\\\",\\\"G\\\":[\\\"$51\\\",[\\\"$\\\",\\\"$L6\\\",\\\"ge-svn\\\",{\\\"type\\\":\\\"global-error\\\",\\\"pagePath\\\":\\\"__next_builtin__global-error.js\\\",\\\"children\\\":[]},null,\\\"$52\\\",0]],\\\"s\\\":false,\\\"S\\\":false}\\n\"])self.__next_f.push([1,\"43:[[\\\"$\\\",\\\"meta\\\",\\\"0\\\",{\\\"charSet\\\":\\\"utf-8\\\"},\\\"$35\\\",\\\"$53\\\",0],[\\\"$\\\",\\\"meta\\\",\\\"1\\\",{\\\"name\\\":\\\"viewport\\\",\\\"content\\\":\\\"width=device-width, initial-scale=1\\\"},\\\"$35\\\",\\\"$54\\\",0]]\\n34:null\\n3b:{\\\"metadata\\\":[[\\\"$\\\",\\\"title\\\",\\\"0\\\",{\\\"children\\\":\\\"GoGoMail Admin Console\\\"},\\\"$38\\\",\\\"$55\\\",0],[\\\"$\\\",\\\"meta\\\",\\\"1\\\",{\\\"name\\\":\\\"description\\\",\\\"content\\\":\\\"Enterprise email administration platform\\\"},\\\"$38\\\",\\\"$56\\\",0],[\\\"$\\\",\\\"link\\\",\\\"2\\\",{\\\"rel\\\":\\\"icon\\\",\\\"href\\\":\\\"/favicon.ico\\\"},\\\"$38\\\",\\\"$57\\\",0],[\\\"$\\\",\\\"$L59\\\",\\\"3\\\",{},\\\"$38\\\",\\\"$58\\\",0]],\\\"error\\\":null,\\\"digest\\\":\\\"$undefined\\\"}\\n4e:\\\"$3b:metadata\\\"\\n\"])Admin ConsoleDashboardSystemQueue StatsBackpressureAPI HealthTenancyCompaniesDomainsDomain SettingsUsers & AccessUsersAdmin UsersDirectoryAliasesDelegationsGroup MembershipsDelivery & MailDelivery RoutesTrusted RelaysMail Flow LogsOutbox EventsDelivery AttemptsSecurityAPI KeysDKIM KeysAudit LogsSuppression ListAlert RulesStorage & QuotasQuota UsageQuota AlertsAttachmentsDriveQuota ReconciliationAnalyticsAPI UsagePush NotificationsReportsConfigurationCompany ConfigDomain ConfigUser ConfigOrganizationSettingsRolesComplianceEnglishDashboardSystem overview and key metricsSystem MetricsTotal Users150Active Domains25API Requests1523Error Rate0.8%System HealthOverall Health: 98%98%Overall Health: 98%: 98%API Server● HealthyDatabase● HealthyMail Queue● HealthyCache● HealthyUptime99.98%Response Time148msStorage UsageStorage Usage: 650/1000 GB65%Storage Usage: 650/1000 GB: 65%Usage65.0%Available350.0GBStatus✓ HealthyQuick ActionsUsersDomainsAudit LogsAPI KeysStorage UsageHealthOverall Health: 98% : 98%Storage Usage: 650/1000 GB : 65%"
```

# Page snapshot

```yaml
- generic [ref=e1]:
  - generic [active]:
    - menu "Next.js Dev Tools Items" [ref=e2]:
      - generic [ref=e3]:
        - menuitem "Route Static" [ref=e4] [cursor=pointer]:
          - generic [ref=e5]: Route
          - generic [ref=e6]: Static
        - menuitem "Try Turbopack" [ref=e7]:
          - generic [ref=e8]: Try Turbopack
          - img [ref=e10]
        - menuitem "Route Info" [ref=e12]:
          - generic [ref=e13]: Route Info
          - img [ref=e15]
      - menuitem "Preferences" [ref=e18]:
        - generic [ref=e19]: Preferences
        - img [ref=e21]
    - button "Close Next.js Dev Tools" [expanded] [ref=e28] [cursor=pointer]:
      - img [ref=e29]
  - alert [ref=e33]
  - main [ref=e35]:
    - navigation [ref=e38]:
      - generic [ref=e39]:
        - button [ref=e41] [cursor=pointer]:
          - generic [ref=e42]:
            - img
        - generic [ref=e43]:
          - heading "Admin Console" [level=2] [ref=e44]:
            - link "Admin Console" [ref=e45] [cursor=pointer]:
              - /url: /companies/default
              - generic [ref=e46]: Admin Console
          - list [ref=e48]:
            - listitem [ref=e49]:
              - link "Dashboard" [ref=e50] [cursor=pointer]:
                - /url: /companies/default/dashboard
            - listitem [ref=e51]:
              - generic [ref=e52]:
                - button "System" [expanded] [ref=e56] [cursor=pointer]:
                  - generic [ref=e58]:
                    - img
                  - generic [ref=e59]: System
                - group "System" [ref=e60]:
                  - list [ref=e61]:
                    - listitem [ref=e62]:
                      - link "Queue Stats" [ref=e63] [cursor=pointer]:
                        - /url: /companies/default/system/queue
                    - listitem [ref=e64]:
                      - link "Backpressure" [ref=e65] [cursor=pointer]:
                        - /url: /companies/default/system/backpressure
                    - listitem [ref=e66]:
                      - link "API Health" [ref=e67] [cursor=pointer]:
                        - /url: /companies/default/system/health
            - listitem [ref=e68]:
              - generic [ref=e69]:
                - button "Tenancy" [expanded] [ref=e73] [cursor=pointer]:
                  - generic [ref=e75]:
                    - img
                  - generic [ref=e76]: Tenancy
                - group "Tenancy" [ref=e77]:
                  - list [ref=e78]:
                    - listitem [ref=e79]:
                      - link "Companies" [ref=e80] [cursor=pointer]:
                        - /url: /companies/default/tenancy/companies
                    - listitem [ref=e81]:
                      - link "Domains" [ref=e82] [cursor=pointer]:
                        - /url: /companies/default/tenancy/domains
                    - listitem [ref=e83]:
                      - link "Domain Settings" [ref=e84] [cursor=pointer]:
                        - /url: /companies/default/tenancy/domain-settings
            - listitem [ref=e85]:
              - generic [ref=e86]:
                - button "Users & Access" [expanded] [ref=e90] [cursor=pointer]:
                  - generic [ref=e92]:
                    - img
                  - generic [ref=e93]: Users & Access
                - group "Users & Access" [ref=e94]:
                  - list [ref=e95]:
                    - listitem [ref=e96]:
                      - link "Users" [ref=e97] [cursor=pointer]:
                        - /url: /companies/default/users
                    - listitem [ref=e98]:
                      - link "Admin Users" [ref=e99] [cursor=pointer]:
                        - /url: /companies/default/admin-users
                    - listitem [ref=e100]:
                      - link "Directory" [ref=e101] [cursor=pointer]:
                        - /url: /companies/default/access/directory
                    - listitem [ref=e102]:
                      - link "Aliases" [ref=e103] [cursor=pointer]:
                        - /url: /companies/default/access/aliases
                    - listitem [ref=e104]:
                      - link "Delegations" [ref=e105] [cursor=pointer]:
                        - /url: /companies/default/access/delegations
                    - listitem [ref=e106]:
                      - link "Group Memberships" [ref=e107] [cursor=pointer]:
                        - /url: /companies/default/access/groups
            - listitem [ref=e108]:
              - generic [ref=e109]:
                - button "Delivery & Mail" [expanded] [ref=e113] [cursor=pointer]:
                  - generic [ref=e115]:
                    - img
                  - generic [ref=e116]: Delivery & Mail
                - group "Delivery & Mail" [ref=e117]:
                  - list [ref=e118]:
                    - listitem [ref=e119]:
                      - link "Delivery Routes" [ref=e120] [cursor=pointer]:
                        - /url: /companies/default/delivery/routes
                    - listitem [ref=e121]:
                      - link "Trusted Relays" [ref=e122] [cursor=pointer]:
                        - /url: /companies/default/delivery/relays
                    - listitem [ref=e123]:
                      - link "Mail Flow Logs" [ref=e124] [cursor=pointer]:
                        - /url: /companies/default/mail/flow-logs
                    - listitem [ref=e125]:
                      - link "Outbox Events" [ref=e126] [cursor=pointer]:
                        - /url: /companies/default/mail/outbox
                    - listitem [ref=e127]:
                      - link "Delivery Attempts" [ref=e128] [cursor=pointer]:
                        - /url: /companies/default/mail/delivery-attempts
            - listitem [ref=e129]:
              - generic [ref=e130]:
                - button "Security" [expanded] [ref=e134] [cursor=pointer]:
                  - generic [ref=e136]:
                    - img
                  - generic [ref=e137]: Security
                - group "Security" [ref=e138]:
                  - list [ref=e139]:
                    - listitem [ref=e140]:
                      - link "API Keys" [ref=e141] [cursor=pointer]:
                        - /url: /companies/default/security/api-keys
                    - listitem [ref=e142]:
                      - link "DKIM Keys" [ref=e143] [cursor=pointer]:
                        - /url: /companies/default/security/dkim-keys
                    - listitem [ref=e144]:
                      - link "Audit Logs" [ref=e145] [cursor=pointer]:
                        - /url: /companies/default/audit-logs
                    - listitem [ref=e146]:
                      - link "Suppression List" [ref=e147] [cursor=pointer]:
                        - /url: /companies/default/security/suppression
                    - listitem [ref=e148]:
                      - link "Alert Rules" [ref=e149] [cursor=pointer]:
                        - /url: /companies/default/security/alerts
            - listitem [ref=e150]:
              - generic [ref=e151]:
                - button "Storage & Quotas" [expanded] [ref=e155] [cursor=pointer]:
                  - generic [ref=e157]:
                    - img
                  - generic [ref=e158]: Storage & Quotas
                - group "Storage & Quotas" [ref=e159]:
                  - list [ref=e160]:
                    - listitem [ref=e161]:
                      - link "Quota Usage" [ref=e162] [cursor=pointer]:
                        - /url: /companies/default/storage/quota-usage
                    - listitem [ref=e163]:
                      - link "Quota Alerts" [ref=e164] [cursor=pointer]:
                        - /url: /companies/default/storage/quota-alerts
                    - listitem [ref=e165]:
                      - link "Attachments" [ref=e166] [cursor=pointer]:
                        - /url: /companies/default/storage/attachments
                    - listitem [ref=e167]:
                      - link "Drive" [ref=e168] [cursor=pointer]:
                        - /url: /companies/default/storage/drive
                    - listitem [ref=e169]:
                      - link "Quota Reconciliation" [ref=e170] [cursor=pointer]:
                        - /url: /companies/default/storage/reconciliation
            - listitem [ref=e171]:
              - generic [ref=e172]:
                - button "Analytics" [expanded] [ref=e176] [cursor=pointer]:
                  - generic [ref=e178]:
                    - img
                  - generic [ref=e179]: Analytics
                - group "Analytics" [ref=e180]:
                  - list [ref=e181]:
                    - listitem [ref=e182]:
                      - link "API Usage" [ref=e183] [cursor=pointer]:
                        - /url: /companies/default/analytics/api-usage
                    - listitem [ref=e184]:
                      - link "Push Notifications" [ref=e185] [cursor=pointer]:
                        - /url: /companies/default/analytics/push
                    - listitem [ref=e186]:
                      - link "Reports" [ref=e187] [cursor=pointer]:
                        - /url: /companies/default/reports
            - listitem [ref=e188]:
              - generic [ref=e189]:
                - button "Configuration" [expanded] [ref=e193] [cursor=pointer]:
                  - generic [ref=e195]:
                    - img
                  - generic [ref=e196]: Configuration
                - group "Configuration" [ref=e197]:
                  - list [ref=e198]:
                    - listitem [ref=e199]:
                      - link "Company Config" [ref=e200] [cursor=pointer]:
                        - /url: /companies/default/config/company
                    - listitem [ref=e201]:
                      - link "Domain Config" [ref=e202] [cursor=pointer]:
                        - /url: /companies/default/config/domain
                    - listitem [ref=e203]:
                      - link "User Config" [ref=e204] [cursor=pointer]:
                        - /url: /companies/default/config/user
            - listitem [ref=e205]:
              - generic [ref=e206]:
                - button "Organization" [expanded] [ref=e210] [cursor=pointer]:
                  - generic [ref=e212]:
                    - img
                  - generic [ref=e213]: Organization
                - group "Organization" [ref=e214]:
                  - list [ref=e215]:
                    - listitem [ref=e216]:
                      - link "Settings" [ref=e217] [cursor=pointer]:
                        - /url: /companies/default/organization
                    - listitem [ref=e218]:
                      - link "Roles" [ref=e219] [cursor=pointer]:
                        - /url: /companies/default/roles
                    - listitem [ref=e220]:
                      - link "Compliance" [ref=e221] [cursor=pointer]:
                        - /url: /companies/default/compliance
    - button "English" [ref=e228]:
      - generic [ref=e229]: English
      - generic [ref=e231]:
        - img
    - generic [ref=e233]:
      - generic [ref=e236]:
        - heading "Dashboard" [level=1] [ref=e239]
        - paragraph [ref=e240]: System overview and key metrics
      - generic [ref=e242]:
        - generic [ref=e245]:
          - heading "System Metrics" [level=2] [ref=e250]
          - generic [ref=e253]:
            - generic:
              - generic [ref=e258]:
                - term [ref=e259]:
                  - generic [ref=e260]: Total Users
                - definition [ref=e261]: "150"
              - generic [ref=e266]:
                - term [ref=e267]:
                  - generic [ref=e268]: Active Domains
                - definition [ref=e269]: "25"
              - generic [ref=e274]:
                - term [ref=e275]:
                  - generic [ref=e276]: API Requests
                - definition [ref=e277]: "1523"
              - generic [ref=e282]:
                - term [ref=e283]:
                  - generic [ref=e284]: Error Rate
                - definition [ref=e285]: 0.8%
        - generic [ref=e287]:
          - generic:
            - generic [ref=e290]:
              - heading "System Health" [level=3] [ref=e295]
              - generic [ref=e298]:
                - generic [ref=e302]:
                  - generic [ref=e303]: "Overall Health: 98%"
                  - generic [ref=e305]:
                    - 'progressbar "Overall Health: 98%" [ref=e306]'
                    - generic [ref=e308]: 98%
                - generic [ref=e312]:
                  - generic [ref=e313]:
                    - term [ref=e314]:
                      - generic [ref=e315]: API Server
                    - definition [ref=e316]: ● Healthy
                  - generic [ref=e317]:
                    - term [ref=e318]:
                      - generic [ref=e319]: Database
                    - definition [ref=e320]: ● Healthy
                  - generic [ref=e321]:
                    - term [ref=e322]:
                      - generic [ref=e323]: Mail Queue
                    - definition [ref=e324]: ● Healthy
                  - generic [ref=e325]:
                    - term [ref=e326]:
                      - generic [ref=e327]: Cache
                    - definition [ref=e328]: ● Healthy
                  - generic [ref=e329]:
                    - term [ref=e330]:
                      - generic [ref=e331]: Uptime
                    - definition [ref=e332]: 99.98%
                  - generic [ref=e333]:
                    - term [ref=e334]:
                      - generic [ref=e335]: Response Time
                    - definition [ref=e336]: 148ms
            - generic [ref=e339]:
              - heading "Storage Usage" [level=3] [ref=e344]
              - generic [ref=e347]:
                - generic [ref=e351]:
                  - generic [ref=e352]: "Storage Usage: 650/1000 GB"
                  - generic [ref=e354]:
                    - 'progressbar "Storage Usage: 650/1000 GB" [ref=e355]'
                    - generic [ref=e357]: 65%
                - generic [ref=e361]:
                  - generic [ref=e362]:
                    - term [ref=e363]:
                      - generic [ref=e364]: Usage
                    - definition [ref=e365]: 65.0%
                  - generic [ref=e366]:
                    - term [ref=e367]:
                      - generic [ref=e368]: Available
                    - definition [ref=e369]: 350.0GB
                  - generic [ref=e370]:
                    - term [ref=e371]:
                      - generic [ref=e372]: Status
                    - definition [ref=e373]: ✓ Healthy
        - generic [ref=e376]:
          - heading "Quick Actions" [level=3] [ref=e381]
          - generic [ref=e384]:
            - generic:
              - link "Users" [ref=e387] [cursor=pointer]:
                - /url: /companies/default/users
              - link "Domains" [ref=e390] [cursor=pointer]:
                - /url: /companies/default/tenancy/domains
              - link "Audit Logs" [ref=e393] [cursor=pointer]:
                - /url: /companies/default/audit-logs
              - link "API Keys" [ref=e396] [cursor=pointer]:
                - /url: /companies/default/security/api-keys
              - link "Storage Usage" [ref=e399] [cursor=pointer]:
                - /url: /companies/default/storage/quota-usage
              - link "Health" [ref=e402] [cursor=pointer]:
                - /url: /companies/default/system/health
  - generic [ref=e403]: "Overall Health: 98% : 98%"
  - generic [ref=e404]: "Storage Usage: 650/1000 GB : 65%"
```

# Test source

```ts
  1  | import { test, expect } from '@playwright/test';
  2  | 
  3  | test('Verify Korean i18n translations on Dashboard', async ({ page }) => {
  4  |   const BASE_URL = 'http://localhost:3001';
  5  | 
  6  |   // Login
  7  |   await page.goto(`${BASE_URL}/login`);
  8  |   await page.fill('input[type="email"]', 'admin@system');
  9  |   await page.fill('input[type="password"]', 'admin1234');
  10 |   await page.click('button:has-text("Sign in")');
  11 |   await page.waitForURL('**/dashboard', { timeout: 15000 });
  12 | 
  13 |   // Change language to Korean
  14 |   await page.click('button:visible >> nth=0');
  15 |   await page.waitForTimeout(200);
  16 |   
  17 |   // Find and click Korean option
  18 |   const koreanOption = page.locator('text=한국어');
  19 |   if (await koreanOption.isVisible()) {
  20 |     await koreanOption.click();
  21 |     await page.waitForTimeout(1000);
  22 |   }
  23 | 
  24 |   // Get page content
  25 |   const pageText = await page.textContent('body');
  26 | 
  27 |   // Check Korean translations
  28 |   console.log('\n📋 검증 중:');
  29 |   const checks = [
  30 |     { text: '대시보드', label: 'Dashboard' },
  31 |     { text: '시스템 지표', label: 'System Metrics' },
  32 |     { text: '총 사용자', label: 'Total Users' },
  33 |     { text: '활성 도메인', label: 'Active Domains' },
  34 |     { text: '빠른 작업', label: 'Quick Actions' },
  35 |   ];
  36 | 
  37 |   let passCount = 0;
  38 |   for (const check of checks) {
  39 |     const found = pageText?.includes(check.text) || false;
  40 |     const status = found ? '✅' : '❌';
  41 |     console.log(`${status} ${check.text} (${check.label})`);
  42 |     if (found) passCount++;
  43 |   }
  44 | 
  45 |   console.log(`\n결과: ${passCount}/${checks.length} 통과\n`);
  46 | 
  47 |   // Assert at least some Korean text is present
> 48 |   expect(pageText).toContain('대시보드');
     |                    ^ Error: expect(received).toContain(expected) // indexOf
  49 |   expect(pageText).toContain('시스템 지표');
  50 | });
  51 | 
```