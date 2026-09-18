import{d as h,i as b,o as s,c,n as m,f as t,h as _,j as x,k as y,l as r,F as k,m as g,a as l,R as w,w as C,t as v,r as L,u as $,p as M}from"./index-Vckk0faO.js";import{c as d}from"./createLucideIcon-SNJDgvBM.js";import{L as I,U as B,F as R}from"./users-gB7JIZB0.js";import{K as V}from"./key-DOJVFUqF.js";/**
 * @license lucide-vue-next v0.400.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const A=d("Building2Icon",[["path",{d:"M6 22V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v18Z",key:"1b4qmf"}],["path",{d:"M6 12H4a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2",key:"i71pzd"}],["path",{d:"M18 9h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2h-2",key:"10jefs"}],["path",{d:"M10 6h4",key:"1itunk"}],["path",{d:"M10 10h4",key:"tcdvrf"}],["path",{d:"M10 14h4",key:"kelpxr"}],["path",{d:"M10 18h4",key:"1ulq68"}]]);/**
 * @license lucide-vue-next v0.400.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const F=d("ChevronLeftIcon",[["path",{d:"m15 18-6-6 6-6",key:"1wnfg3"}]]);/**
 * @license lucide-vue-next v0.400.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const q=d("ChevronRightIcon",[["path",{d:"m9 18 6-6-6-6",key:"mthhwq"}]]);/**
 * @license lucide-vue-next v0.400.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const z=d("LogOutIcon",[["path",{d:"M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4",key:"1uf3rs"}],["polyline",{points:"16 17 21 12 16 7",key:"1gabdz"}],["line",{x1:"21",x2:"9",y1:"12",y2:"12",key:"1uyos4"}]]),D={class:"flex items-center h-14 px-4 border-b"},H={key:0,class:"font-semibold text-lg"},K={class:"flex-1 p-2 space-y-1"},N={key:0},S=h({__name:"Sidebar",setup(p){const a=b(),e=L(!1),u=[{path:"/dashboard",label:"Dashboard",icon:I},{path:"/contacts",label:"Contacts",icon:B},{path:"/tenants",label:"Tenants",icon:A},{path:"/forms",label:"Forms",icon:R},{path:"/api-keys",label:"API Keys",icon:V}],i=o=>a.path===o||a.path.startsWith(o+"/");return(o,f)=>(s(),c("aside",{class:m(["flex flex-col border-r bg-card transition-all duration-200",e.value?"w-16":"w-56"])},[t("div",D,[e.value?_("",!0):(s(),c("span",H,"Automata")),t("button",{onClick:f[0]||(f[0]=n=>e.value=!e.value),class:"ml-auto p-1 hover:bg-accent rounded"},[(s(),x(y(e.value?r(q):r(F)),{class:"w-4 h-4"}))])]),t("nav",K,[(s(),c(k,null,g(u,n=>l(r(w),{key:n.path,to:n.path,class:m(["flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors",i(n.path)?"bg-accent":"hover:bg-accent"])},{default:C(()=>[(s(),x(y(n.icon),{class:"w-4 h-4 flex-shrink-0"})),e.value?_("",!0):(s(),c("span",N,v(n.label),1))]),_:2},1032,["to","class"])),64))])],2))}}),j={class:"flex items-center h-14 px-6 border-b bg-card"},O={class:"flex items-center gap-4"},T={class:"text-sm text-muted-foreground"},U=h({__name:"Header",setup(p){const a=$();async function e(){await a.logout()}return(u,i)=>{var o;return s(),c("header",j,[i[0]||(i[0]=t("div",{class:"flex-1"},null,-1)),t("div",O,[t("span",T,v((o=r(a).user)==null?void 0:o.email),1),t("button",{onClick:e,class:"p-2 hover:bg-accent rounded-md transition-colors",title:"Logout"},[l(r(z),{class:"w-4 h-4"})])])])}}}),E={class:"flex h-screen bg-muted/20"},G={class:"flex-1 flex flex-col overflow-hidden"},P={class:"flex-1 overflow-y-auto p-6"},X=h({__name:"AppLayout",setup(p){return(a,e)=>(s(),c("div",E,[l(S),t("div",G,[l(U),t("main",P,[l(r(M))])])]))}});export{X as default};
