import { BrowserRouter } from "react-router-dom";
import { DemoProvider } from "./contexts/DemoContext";
import { AppRoutes } from "./routes/AppRoutes";

function App() {
  return (
    <BrowserRouter>
      <DemoProvider>
        <AppRoutes />
      </DemoProvider>
    </BrowserRouter>
  );
}

export default App;
