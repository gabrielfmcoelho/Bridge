import { redirect } from "next/navigation";

// "Credenciais de host" are a kind of vault credential now (v91 moved the
// ssh_keys library into the vault): old links land on the filtered Cofre.
export default function HostCredentialsRedirectPage() {
  redirect("/secrets?kind=host_cred");
}
