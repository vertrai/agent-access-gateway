const $=id=>document.getElementById(id);
const managerAdminKeyStorage='managerAdminKey';
function currentManagerAdminKey(){const input=$('adminKey');return input?input.value.trim():sessionStorage.getItem(managerAdminKeyStorage)||''}
function adminHeaders(){return {'Authorization':'Bearer '+currentManagerAdminKey(),'Content-Type':'application/json'}}
async function adminRequest(path,options={}){const response=await fetch(path,{...options,headers:{...adminHeaders(),...(options.headers||{})}});const data=await response.json().catch(()=>({}));if(!response.ok){const error=new Error(data.error||`HTTP ${response.status}`);error.status=response.status;error.data=data;throw error}return data}
function initAdminKey(){const input=$('adminKey');if(!input||input.dataset.initialized)return;input.dataset.initialized='true';const legacy=sessionStorage.getItem('gatewayAdminKey')||'';const saved=sessionStorage.getItem(managerAdminKeyStorage)||legacy;if(legacy&&!sessionStorage.getItem(managerAdminKeyStorage)){sessionStorage.setItem(managerAdminKeyStorage,legacy);sessionStorage.removeItem('gatewayAdminKey')}input.value=saved;const save=()=>{sessionStorage.setItem(managerAdminKeyStorage,input.value.trim());document.dispatchEvent(new CustomEvent('manager-admin-key-change'))};input.addEventListener('input',save);input.addEventListener('change',save)}
function esc(value){return String(value??'').replace(/[&<>'"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]))}
function pill(status){return `<span class="pill ${esc(status)}">${esc(status||'-')}</span>`}
function setBusy(button,busy,text='处理中…'){if(!button)return;if(!button.dataset.label)button.dataset.label=button.textContent;button.disabled=busy;button.textContent=busy?text:button.dataset.label}
function showStatus(node,message,ok=false){node.textContent=message;node.className='status '+(ok?'success':'error')}
document.addEventListener('DOMContentLoaded',initAdminKey);
