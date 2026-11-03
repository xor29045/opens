import { Box, Button, Container, HStack, Link as CLink, Text } from '@chakra-ui/react';
import { useStore } from '@nanostores/react';
import { sessionAtom, homeFor } from '../lib/auth/session';
import Providers from './Providers';

// Public navigation. Each Astro island is its own React root, so it carries
// its providers. Shows "Mon espace" only when a session exists in memory.
export default function PublicNavbar() {
  return (
    <Providers>
      <NavbarInner />
    </Providers>
  );
}

function NavbarInner() {
  const session = useStore(sessionAtom);
  return (
    <Box
      as="header"
      position="sticky"
      top="0"
      zIndex="sticky"
      borderBottomWidth="1px"
      borderColor="border"
      bg="rgba(12, 15, 20, 0.86)"
      backdropFilter="blur(10px)"
    >
      <Container maxW="1240px" px="5">
        <HStack justify="space-between" h="16" gap="6">
          <HStack gap="8">
            <a href="/" className="os-wordmark" aria-label="Opensex — accueil">
              Open<span>•</span>sex
            </a>
            <HStack as="nav" aria-label="Navigation principale" gap="6" display={{ base: 'none', md: 'flex' }}>
              <CLink href="/client/espace/decouvrir" fontSize="sm" color="fg.muted" _hover={{ color: 'fg', textDecoration: 'none' }}>
                Découvrir
              </CLink>
              <CLink href="/gerant/register" fontSize="sm" color="fg.muted" _hover={{ color: 'fg', textDecoration: 'none' }}>
                Vous êtes une maison ?
              </CLink>
            </HStack>
          </HStack>
          <HStack gap="2">
            {session.user ? (
              <Button asChild size="sm" colorPalette="brand">
                <a href={homeFor[session.user.role]}>Mon espace</a>
              </Button>
            ) : (
              <>
                <Button asChild size="sm" variant="ghost" color="fg.muted">
                  <a href="/client/login">
                    <Text as="span">Connexion</Text>
                  </a>
                </Button>
                <Button asChild size="sm" colorPalette="brand">
                  <a href="/client/register">Créer un compte</a>
                </Button>
              </>
            )}
          </HStack>
        </HStack>
      </Container>
    </Box>
  );
}
