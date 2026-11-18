import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { AllCommunityModule, ModuleRegistry } from 'ag-grid-community'
import '@fontsource-variable/roboto-flex'
import 'material-symbols/outlined.css'
import './styles/tokens.css'
import './styles/app.css'
import App from './App'

ModuleRegistry.registerModules([AllCommunityModule])

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
